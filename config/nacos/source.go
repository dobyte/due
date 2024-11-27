package nacos

import (
	"context"
	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const Name = "nacos"

type Source struct {
	err      error
	opts     *options
	ctx      context.Context
	cancel   context.CancelFunc
	builtin  bool
	version  uint64
	versions map[string]uint64
	chListen chan string
	chCancel chan string
	rw       sync.RWMutex
	watchers map[*watcher]struct{}
}

func NewSource(opts ...Option) config.Source {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	s := &Source{}
	s.opts = o
	s.ctx, s.cancel = context.WithCancel(o.ctx)
	s.versions = make(map[string]uint64)
	s.chListen = make(chan string)
	s.chCancel = make(chan string)
	s.watchers = make(map[*watcher]struct{})

	if o.client == nil {
		o.client, s.err = s.buildClient()
		s.builtin = true
	}

	go s.listen()
	go s.refresh()

	return s
}

// Name 配置源名称
func (s *Source) Name() string {
	return Name
}

// Load 加载配置项
func (s *Source) Load(ctx context.Context, file ...string) ([]*config.Configuration, error) {
	if s.err != nil {
		return nil, s.err
	}

	if len(file) > 0 {
		content, err := s.opts.client.GetConfig(vo.ConfigParam{
			DataId: file[0],
			Group:  s.opts.groupName,
		})
		if err != nil {
			return nil, err
		}

		configuration := conv(file[0], content)

		return []*config.Configuration{configuration}, nil
	} else {
		index := 1
		configurations := make([]*config.Configuration, 0)

		for {
			result, err := s.opts.client.SearchConfig(vo.SearchConfigParam{
				Search:   "blur",
				Group:    s.opts.groupName,
				PageNo:   index,
				PageSize: 10,
			})
			if err != nil {
				log.Warnf("search config list failed: %v", err)
				break
			}

			for _, item := range result.PageItems {
				configurations = append(configurations, conv(item.DataId, item.Content))
			}

			if result.PageNumber >= result.PagesAvailable {
				break
			}

			index = result.PageNumber + 1
		}

		return configurations, nil
	}
}

// Store 保存配置项
func (s *Source) Store(ctx context.Context, file string, content []byte) error {
	if s.err != nil {
		return s.err
	}

	if s.opts.mode != config.WriteOnly && s.opts.mode != config.ReadWrite {
		return errors.ErrNoOperationPermission
	}

	data := string(content)

	ok, err := s.opts.client.PublishConfig(vo.ConfigParam{
		DataId:  file,
		Group:   s.opts.groupName,
		Content: data,
	})
	if err != nil {
		return err
	}

	if ok {
		s.onChange(s.opts.namespaceId, s.opts.groupName, file, data)
	}

	return nil
}

// Watch 监听配置项
func (s *Source) Watch(ctx context.Context) (config.Watcher, error) {
	if s.err != nil {
		return nil, s.err
	}

	w, err := newWatcher(ctx, s)
	if err != nil {
		return nil, err
	}

	s.rw.Lock()
	s.watchers[w] = struct{}{}
	s.rw.Unlock()

	return w, nil
}

// Close 关闭配置源
func (s *Source) Close() error {
	if s.err != nil {
		return s.err
	}

	if s.builtin {
		s.opts.client.CloseClient()
	}

	s.cancel()
	close(s.chListen)
	close(s.chCancel)

	return nil
}

// 监听dataId
func (s *Source) listen() {
	if s.err != nil {
		return
	}

	for {
		select {
		case <-s.ctx.Done():
			return
		case dataId, ok := <-s.chListen:
			if !ok {
				return
			}

			if err := s.opts.client.ListenConfig(vo.ConfigParam{
				DataId:   dataId,
				Group:    s.opts.groupName,
				OnChange: s.onChange,
			}); err != nil {
				log.Warnf("%s %s listen failed: %v", s.opts.groupName, dataId, err)
			}
		case dataId, ok := <-s.chCancel:
			if !ok {
				return
			}

			if err := s.opts.client.CancelListenConfig(vo.ConfigParam{
				DataId: dataId,
				Group:  s.opts.groupName,
			}); err != nil {
				log.Warnf("%s %s cancel listen failed: %v", s.opts.groupName, dataId, err)
			}
		}
	}
}

// 刷新dataId
func (s *Source) refresh() {
	if s.err != nil {
		return
	}

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.search()
		}
	}
}

func (s *Source) search() {
	s.version++

	index := 1

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			// ignore
		}

		result, err := s.opts.client.SearchConfig(vo.SearchConfigParam{
			Search:   "blur",
			Group:    s.opts.groupName,
			PageNo:   index,
			PageSize: 10,
		})
		if err != nil {
			log.Warnf("search config list failed: %v", err)
			break
		}

		for _, item := range result.PageItems {
			if _, ok := s.versions[item.DataId]; !ok {
				s.chListen <- item.DataId
			}

			s.versions[item.DataId] = s.version
		}

		if result.PageNumber >= result.PagesAvailable {
			break
		}

		index = result.PageNumber + 1
	}

	for dataId, version := range s.versions {
		if version != s.version {
			s.chCancel <- dataId
		}
	}
}

func (s *Source) onChange(_, _, file, content string) {
	configuration := conv(file, content)

	s.rw.RLock()
	for w := range s.watchers {
		w.notice(configuration)
	}
	s.rw.RUnlock()
}

// 构建客户端
func (s *Source) buildClient() (config_client.IConfigClient, error) {
	param := vo.NacosClientParam{
		ServerConfigs: make([]constant.ServerConfig, 0, len(s.opts.urls)),
		ClientConfig: &constant.ClientConfig{
			TimeoutMs:            uint64(s.opts.timeout.Microseconds()),
			NamespaceId:          s.opts.namespaceId,
			Endpoint:             s.opts.endpoint,
			RegionId:             s.opts.regionId,
			AccessKey:            s.opts.accessKey,
			SecretKey:            s.opts.secretKey,
			OpenKMS:              s.opts.openKMS,
			CacheDir:             s.opts.cacheDir,
			Username:             s.opts.username,
			Password:             s.opts.password,
			LogDir:               s.opts.logDir,
			LogLevel:             s.opts.logLevel,
			NotLoadCacheAtStart:  true,
			UpdateCacheWhenEmpty: true,
		},
	}

	var (
		err      error
		endpoint string
	)

	for _, v := range s.opts.urls {
		if raw, e := url.Parse(v); e != nil {
			err, endpoint = e, v
		} else {
			host, p, e := net.SplitHostPort(raw.Host)
			if err != nil {
				err, endpoint = e, v
				continue
			}

			port, e := strconv.ParseUint(p, 10, 64)
			if e != nil {
				err, endpoint = e, v
				continue
			}

			param.ServerConfigs = append(param.ServerConfigs, constant.ServerConfig{
				Scheme:      raw.Scheme,
				ContextPath: raw.Path,
				IpAddr:      host,
				Port:        port,
			})
		}
	}

	if len(param.ServerConfigs) == 0 {
		if err != nil {
			return nil, err
		} else {
			return nil, errors.New("invalid server urls")
		}
	} else {
		if err != nil {
			log.Warnf("%s parse failed: %v", endpoint, err)
		}

		return clients.NewConfigClient(param)
	}
}

func conv(file, content string) *config.Configuration {
	ext := filepath.Ext(file)

	return &config.Configuration{
		File:     file,
		Name:     strings.TrimSuffix(file, ext),
		Format:   strings.TrimPrefix(ext, "."),
		Content:  []byte(content),
		FullPath: file,
	}
}
