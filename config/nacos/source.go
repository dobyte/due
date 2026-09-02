package nacos

import (
	"context"
	"net"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/task"
	"github.com/nacos-group/nacos-sdk-go/v2/clients"
	"github.com/nacos-group/nacos-sdk-go/v2/clients/config_client"
	"github.com/nacos-group/nacos-sdk-go/v2/common/constant"
	"github.com/nacos-group/nacos-sdk-go/v2/vo"
)

// Name 配置源名称
const Name = "nacos"

// listenRequest 监听请求
// 携带dataId及用于回传监听结果的通道
type listenRequest struct {
	dataId string     // 配置dataId
	done   chan error // 监听结果回传通道
}

type Source struct {
	err      error              // 构建客户端错误信息
	opts     *options           // 配置项
	ctx      context.Context    // 上下文
	cancel   context.CancelFunc // 取消函数
	builtin  bool               // 是否为内置客户端
	version  uint64             // 当前搜索版本号
	versions map[string]uint64  // 各dataId对应的搜索版本号
	chListen chan listenRequest // 监听指令通道
	chCancel chan string        // 取消监听指令通道
	watchers sync.Map           // 监听器集合
	once     sync.Once          // 保证关闭操作只执行一次
}

// NewSource 创建配置源
// 根据选项构建Nacos配置中心客户端，并启动配置监听与刷新协程；
// 传入外部客户端时优先使用外部客户端，且该客户端由调用方负责关闭
// @param opts ...Option 配置项
// @return @1 config.Source 配置源
func NewSource(opts ...Option) config.Source {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	s := &Source{}
	s.opts = o
	s.ctx, s.cancel = context.WithCancel(o.ctx)
	s.versions = make(map[string]uint64)
	s.chListen = make(chan listenRequest)
	s.chCancel = make(chan string)

	if o.client == nil {
		o.client, s.err = s.buildClient()
		s.builtin = true
	}

	go s.listen()
	go s.refresh()

	return s
}

// Name 获取配置源名称
// @return @1 string 配置源名称
func (s *Source) Name() string {
	return Name
}

// Load 加载配置项
// 传入file参数时仅加载指定的配置项；未传入file参数时，分页查询群组下所有配置项并加载
// @param ctx context.Context 上下文
// @param file ...string 待加载的配置文件(dataId)
// @return @1 []*config.Configuration 配置项列表
// @return @2 error 错误信息
func (s *Source) Load(ctx context.Context, file ...string) ([]*config.Configuration, error) {
	if s.err != nil {
		return nil, s.err
	}

	if len(file) > 0 && file[0] != "" {
		if configuration, err := s.load(file[0]); err != nil {
			return nil, err
		} else {
			return []*config.Configuration{configuration}, nil
		}
	} else {
		var (
			mu             sync.Mutex
			index          = 1
			configurations = make([]*config.Configuration, 0)
		)

		for {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
				// continue
			}

			result, err := s.opts.client.SearchConfig(vo.SearchConfigParam{
				Search:   "blur",
				Group:    s.opts.groupName,
				PageNo:   index,
				PageSize: 20,
			})
			if err != nil {
				return nil, err
			}

			switch len(result.PageItems) {
			case 0:
				// ignore
			case 1:
				if configuration, err := s.load(result.PageItems[0].DataId); err != nil {
					return nil, err
				} else {
					configurations = append(configurations, configuration)
				}
			default:
				wg, _ := task.WithContext(ctx)

				for _, item := range result.PageItems {
					wg.Go(func() error {
						configuration, err := s.load(item.DataId)
						if err != nil {
							return err
						}

						mu.Lock()
						configurations = append(configurations, configuration)
						mu.Unlock()

						return nil
					})
				}

				if err := wg.Wait(); err != nil {
					return nil, err
				}
			}

			if result.PageNumber >= result.PagesAvailable {
				break
			}

			index = result.PageNumber + 1
		}

		return configurations, nil
	}
}

// load 加载单个配置项
// 通过dataId从Nacos服务端拉取配置内容，并转换为统一的配置结构
// @param file string 配置文件(dataId)
// @return @1 *config.Configuration 配置项
// @return @2 error 错误信息
func (s *Source) load(file string) (*config.Configuration, error) {
	content, err := s.opts.client.GetConfig(vo.ConfigParam{
		DataId: file,
		Group:  s.opts.groupName,
	})
	if err != nil {
		return nil, err
	}

	configuration := s.conv(file, content)

	return configuration, nil
}

// Store 保存配置项
// 仅支持write-only和read-write模式；发布成功后由服务端推送变更给已注册的监听器
// @param ctx context.Context 上下文
// @param file string 配置文件(dataId)
// @param content []byte 配置内容
// @return @1 error 错误信息
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
		Type:    s.parseFileType(file),
	})
	if err != nil {
		return err
	}

	if !ok {
		return errors.ErrConfigStoreFailed
	}

	s.onChange(s.opts.namespaceId, s.opts.groupName, file, data)

	return nil
}

// Watch 监听配置项
// 创建新的监听器并注册到配置源中，配置变更时通过监听器通知
// @param ctx context.Context 上下文
// @return @1 config.Watcher 监听器
// @return @2 error 错误信息
func (s *Source) Watch(ctx context.Context) (config.Watcher, error) {
	if s.err != nil {
		return nil, s.err
	}

	w := newWatcher(ctx, s)
	s.watchers.Store(w, struct{}{})

	return w, nil
}

// Close 关闭配置源
// 取消上下文并关闭内置客户端，终止监听与刷新协程；保证关闭操作只执行一次
// @return @1 error 错误信息
func (s *Source) Close() error {
	if s.err != nil {
		return s.err
	}

	// 保证关闭操作只执行一次，避免重复关闭客户端导致panic
	s.once.Do(func() {
		s.cancel()

		if s.builtin {
			s.opts.client.CloseClient()
		}
	})

	return nil
}

// listen 处理监听与取消监听指令
// 消费search循环产生的dataId，分别执行配置监听注册与取消
func (s *Source) listen() {
	if s.err != nil {
		return
	}

	for {
		select {
		case <-s.ctx.Done():
			return
		case req, ok := <-s.chListen:
			if !ok {
				return
			}

			err := s.opts.client.ListenConfig(vo.ConfigParam{
				DataId:   req.dataId,
				Group:    s.opts.groupName,
				OnChange: s.onChange,
			})
			if err != nil {
				log.Warnf("%s %s listen failed: %v", s.opts.groupName, req.dataId, err)
			}

			// 非阻塞回传监听结果，避免请求方已退出时阻塞
			select {
			case req.done <- err:
			default:
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

// refresh 定时刷新配置
// 每隔3秒执行一次配置搜索，检测新增或删除的配置项
func (s *Source) refresh() {
	if s.err != nil {
		return
	}

	s.search()

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

// search 搜索群组下的配置项
// 分页查询配置列表，对新增的dataId发送监听指令，对已删除的dataId发送取消监听指令
func (s *Source) search() {
	s.version++

	index := 1

	for {
		select {
		case <-s.ctx.Done():
			return
		default:
			// continue
		}

		result, err := s.opts.client.SearchConfig(vo.SearchConfigParam{
			Search:   "blur",
			Group:    s.opts.groupName,
			PageNo:   index,
			PageSize: 20,
		})
		if err != nil {
			log.Warnf("search config list failed: %v", err)
			// 查询失败时直接返回，避免版本号已递增而配置未刷新，
			// 导致下方取消循环误判所有配置均已失效而批量取消监听
			return
		}

		for _, item := range result.PageItems {
			if _, ok := s.versions[item.DataId]; !ok {
				// 监听失败时跳过版本标记，留待下次搜索重试
				if err := s.requestListen(item.DataId); err != nil {
					continue
				}
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
			select {
			case s.chCancel <- dataId:
			case <-s.ctx.Done():
				return
			}

			// 取消监听后立即从版本表中移除，
			// 避免重复发送取消指令，同时保证该配置被删除后重新创建时能够再次被监听
			delete(s.versions, dataId)
		}
	}
}

// requestListen 请求监听配置
// 向监听协程发送监听指令并等待监听结果返回
// @param dataId string 配置dataId
// @return @1 error 错误信息
func (s *Source) requestListen(dataId string) error {
	done := make(chan error, 1)
	req := listenRequest{dataId: dataId, done: done}

	select {
	case s.chListen <- req:
	case <-s.ctx.Done():
		return s.ctx.Err()
	}

	select {
	case err := <-done:
		return err
	case <-s.ctx.Done():
		return s.ctx.Err()
	}
}

// onChange 配置变更回调
// Nacos服务端推送配置变更时触发，将变更后的配置通知给所有已注册的监听器
// @param file string 配置文件(dataId)
// @param content string 配置内容
func (s *Source) onChange(_, _, file, content string) {
	configuration := s.conv(file, content)

	s.watchers.Range(func(key, value any) bool {
		w := key.(*watcher)
		w.notice(configuration)
		return true
	})
}

// buildClient 构建Nacos配置客户端
// 解析服务器地址列表并构建Nacos配置中心客户端；地址支持可选的scheme，缺省时默认为http
// @return @1 config_client.IConfigClient Nacos配置客户端
// @return @2 error 错误信息
func (s *Source) buildClient() (config_client.IConfigClient, error) {
	param := vo.NacosClientParam{
		ServerConfigs: make([]constant.ServerConfig, 0, len(s.opts.urls)),
		ClientConfig: &constant.ClientConfig{
			TimeoutMs:            uint64(s.opts.timeout.Milliseconds()),
			NamespaceId:          s.opts.namespaceId,
			ClusterName:          s.opts.clusterName,
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
		if !strings.Contains(v, "://") {
			v = "http://" + v
		}

		raw, e := url.Parse(v)
		if e != nil {
			err, endpoint = e, v
			continue
		}

		host, p, e := net.SplitHostPort(raw.Host)
		if e != nil {
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

// parseFileType 转换配置类型
// 将dataId的文件后缀转换为Nacos支持的配置类型，仅支持json、xml和yaml，
// 其余格式统一转换为默认的text类型
// @param file string 配置文件(dataId)
// @return @1 string Nacos配置类型
func (s *Source) parseFileType(file string) string {
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(file), ".")) {
	case "json":
		return "json"
	case "xml":
		return "xml"
	case "yml", "yaml":
		return "yaml"
	default:
		return "text"
	}
}

// conv 转换配置
// 将dataId和内容转换为统一的配置结构，dataId的文件后缀作为配置格式
// @param file string 配置文件(dataId)
// @param content string 配置内容
// @return @1 *config.Configuration 配置项
func (s *Source) conv(file, content string) *config.Configuration {
	ext := filepath.Ext(file)

	return &config.Configuration{
		File:     file,
		Name:     strings.TrimSuffix(file, ext),
		Format:   strings.TrimPrefix(ext, "."),
		Content:  []byte(content),
		FullPath: file,
	}
}
