package polaris

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/task"
	"github.com/polarismesh/polaris-go/api"
	"github.com/polarismesh/polaris-go/pkg/model"
	apimodel "github.com/polarismesh/specification/source/go/api/v1/model"
)

// Name 配置源名称
const Name = "polaris"

// Source 配置源
type Source struct {
	err            error              // 构建客户端错误信息
	opts           *options           // 配置项
	ctx            context.Context    // 上下文
	cancel         context.CancelFunc // 取消函数
	builtin        bool               // 是否为内置客户端
	version        uint64             // 当前搜索版本号
	versions       map[string]uint64  // 各配置文件对应的搜索版本号
	chListen       chan string        // 监听指令通道
	chCancel       chan string        // 取消监听指令通道
	watchers       sync.Map           // 监听器集合
	once           sync.Once          // 保证关闭操作只执行一次
	searchDisabled bool               // 分组搜索是否不可用
	configClient   api.ConfigFileAPI  // 配置文件客户端
	groupClient    api.ConfigGroupAPI // 配置分组客户端
	subscribed     sync.Map           // 已注册变更监听器的配置文件集合
}

// NewSource 创建配置源
// 根据选项构建Polaris配置中心客户端，并启动配置监听与刷新协程；
// 传入外部SDK上下文时优先使用外部SDK上下文，且该上下文由调用方负责销毁
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
	s.chListen = make(chan string)
	s.chCancel = make(chan string)

	if o.client == nil {
		o.client, s.err = s.buildClient()
		s.builtin = true
	}

	if s.err == nil {
		s.configClient = api.NewConfigFileAPIBySDKContext(o.client)
		s.groupClient = api.NewConfigGroupAPIBySDKContext(o.client)
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
// 传入file参数时仅加载指定的配置项；未传入file参数时，加载分组下所有已发布的配置项
// @param ctx context.Context 上下文
// @param file ...string 待加载的配置文件名称
// @return @1 []*config.Configuration 配置项列表
// @return @2 error 错误信息
func (s *Source) Load(ctx context.Context, file ...string) ([]*config.Configuration, error) {
	if s.err != nil {
		return nil, s.err
	}

	if len(file) > 0 {
		if configuration, err := s.load(file[0]); err != nil {
			return nil, err
		} else {
			return []*config.Configuration{configuration}, nil
		}
	} else {
		var (
			mu             sync.Mutex
			configurations = make([]*config.Configuration, 0)
		)

		group, err := s.groupClient.GetConfigGroup(s.opts.namespace, s.opts.group)
		if err != nil {
			return nil, err
		}

		files, _, ok := group.GetFiles()
		if !ok {
			return configurations, nil
		}

		wg, _ := task.WithContext(ctx)

		for _, item := range files {
			fileName := item.FileName
			wg.Go(func() error {
				configuration, err := s.load(fileName)
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

		return configurations, nil
	}
}

// load 加载单个配置项
// 通过配置文件名称从Polaris服务端拉取配置内容，并转换为统一的配置结构；
// 加载成功后同时订阅该配置文件的变更，保证旧版本服务端（不支持分组查询接口）也能收到变更通知
// @param file string 配置文件名称
// @return @1 *config.Configuration 配置项
// @return @2 error 错误信息
func (s *Source) load(file string) (*config.Configuration, error) {
	configFile, err := s.configClient.GetConfigFile(s.opts.namespace, s.opts.group, file)
	if err != nil {
		return nil, err
	}

	if !configFile.HasContent() {
		return nil, errors.New("config file not exist")
	}

	s.subscribe(file)

	configuration := conv(file, configFile.GetContent())

	return configuration, nil
}

// Store 保存配置项
// 仅支持write-only和read-write模式；优先使用UpsertAndPublishConfigFile一步完成创建或更新并发布，
// 服务端不支持该接口（Unimplemented）或返回数据冲突（DataConflict）时，退化为创建（已存在则更新）后发布，
// 发布成功后由服务端推送变更给已注册的监听器
// @param ctx context.Context 上下文
// @param file string 配置文件名称
// @param content []byte 配置内容
// @return @1 error 错误信息
func (s *Source) Store(ctx context.Context, file string, content []byte) error {
	if s.err != nil {
		return s.err
	}

	if s.opts.mode != config.WriteOnly && s.opts.mode != config.ReadWrite {
		return errors.ErrNoOperationPermission
	}

	if err := s.configClient.UpsertAndPublishConfigFile(s.opts.namespace, s.opts.group, file, string(content)); err == nil {
		return nil
	} else if !isUnimplementedError(err) && !isDataConflictError(err) {
		return err
	}

	if err := s.createOrUpdateConfigFile(file, string(content)); err != nil {
		return err
	}

	return s.configClient.PublishConfigFile(s.opts.namespace, s.opts.group, file)
}

// createOrUpdateConfigFile 创建或更新配置文件
// 先尝试创建配置文件，创建失败且提示资源已存在时转为更新
// @param file string 配置文件名称
// @param content string 配置内容
// @return @1 error 错误信息
func (s *Source) createOrUpdateConfigFile(file, content string) error {
	if err := s.configClient.CreateConfigFile(s.opts.namespace, s.opts.group, file, content); err == nil {
		return nil
	} else if !isExistedResourceError(err) {
		return err
	}

	return s.configClient.UpdateConfigFile(s.opts.namespace, s.opts.group, file, content)
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

	w, err := newWatcher(ctx, s)
	if err != nil {
		return nil, err
	}

	s.watchers.Store(w, struct{}{})

	return w, nil
}

// Close 关闭配置源
// 取消上下文并销毁内置SDK上下文，终止监听与刷新协程；保证关闭操作只执行一次
// @return @1 error 错误信息
func (s *Source) Close() error {
	if s.err != nil {
		return s.err
	}

	// 保证关闭操作只执行一次，避免重复销毁SDK上下文导致panic
	s.once.Do(func() {
		s.cancel()

		if s.builtin {
			s.opts.client.Destroy()
		}
	})

	return nil
}

// listen 处理监听指令
// 消费search循环产生的配置文件名称，为其注册变更监听器
func (s *Source) listen() {
	if s.err != nil {
		return
	}

	for {
		select {
		case <-s.ctx.Done():
			return
		case fileName, ok := <-s.chListen:
			if !ok {
				return
			}

			s.subscribe(fileName)
		case <-s.chCancel:
			// SDK未提供移除文件监听器的接口，配置文件删除后服务端停止推送变更，
			// 已注册的监听器保持不动，待配置文件重新创建后仍可继续收到变更通知
		}
	}
}

// subscribe 订阅配置文件变更
// 为配置文件创建对象并注册变更监听器，同一配置文件只注册一次；
// SDK未提供移除监听器的接口，文件删除后重新创建时已注册的监听器依然有效，
// 因此记录已订阅的配置文件，避免重复注册导致重复通知；
// 首次订阅时立即推送当前配置内容，保证监听器能感知到已发布的最新配置
// （与Nacos的ListenConfig行为一致：注册监听后立即回调一次当前配置）
// @param fileName string 配置文件名称
func (s *Source) subscribe(fileName string) {
	// 原子占位，确保同一文件只由一个协程执行网络订阅与监听器注册，
	// 避免并发场景下重复注册监听器导致重复通知
	if _, loaded := s.subscribed.LoadOrStore(fileName, struct{}{}); loaded {
		return
	}

	req := &api.GetConfigFileRequest{}
	req.Namespace = s.opts.namespace
	req.FileGroup = s.opts.group
	req.FileName = fileName
	req.Subscribe = true

	configFile, err := s.configClient.FetchConfigFile(req)
	if err != nil {
		// 订阅失败时移除占位记录，便于后续搜索周期重新订阅
		s.subscribed.Delete(fileName)
		log.Warnf("%s/%s/%s listen failed: %v", s.opts.namespace, s.opts.group, fileName, err)
		return
	}

	configFile.AddChangeListener(s.onChange)

	s.notify(conv(fileName, configFile.GetContent()))
}

// notify 通知配置变更
// 将变更后的配置通知给所有已注册的监听器
// @param configuration *config.Configuration 配置项
func (s *Source) notify(configuration *config.Configuration) {
	s.watchers.Range(func(key, value any) bool {
		w := key.(*watcher)
		w.notice(configuration)
		return true
	})
}

// refresh 定时刷新配置
// 每隔3秒执行一次配置搜索，检测分组下新增或删除的配置文件
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

// search 搜索分组下的配置文件
// 获取分组下所有已发布的配置文件，对新增的配置文件发送监听指令，对已删除的配置文件发送取消监听指令
func (s *Source) search() {
	if s.searchDisabled {
		return
	}

	s.version++

	group, err := s.groupClient.GetConfigGroup(s.opts.namespace, s.opts.group)
	if err != nil {
		if isUnimplementedError(err) {
			// 服务端不支持分组查询接口（GetConfigFileMetadataList），
			// 禁用分组搜索，仅通过显式加载（Load）订阅配置文件变更
			s.searchDisabled = true
			log.Warnf("search config list is not supported by server, disable group search: %v", err)
		} else {
			log.Warnf("search config list failed: %v", err)
			// 查询失败时直接返回，避免版本号已递增而配置未刷新，
			// 导致下方取消循环误判所有配置均已失效而批量取消监听
		}
		return
	}

	files, _, ok := group.GetFiles()
	if !ok {
		// 分组不存在或分组下无已发布配置，取消全部监听
		for fileName := range s.versions {
			select {
			case s.chCancel <- fileName:
			case <-s.ctx.Done():
				return
			}

			delete(s.versions, fileName)
		}

		return
	}

	for _, item := range files {
		if _, ok := s.versions[item.FileName]; !ok {
			select {
			case s.chListen <- item.FileName:
			case <-s.ctx.Done():
				return
			}
		}

		s.versions[item.FileName] = s.version
	}

	for fileName, version := range s.versions {
		if version != s.version {
			select {
			case s.chCancel <- fileName:
			case <-s.ctx.Done():
				return
			}

			// 取消监听后立即从版本表中移除，
			// 避免重复发送取消指令，同时保证该配置被删除后重新创建时能够再次被监听
			delete(s.versions, fileName)
		}
	}
}

// onChange 配置变更回调
// Polaris服务端推送配置变更时触发，将变更后的配置通知给所有已注册的监听器；
// 配置文件被删除时（Deleted）跳过通知，避免向监听器推送空内容
// @param event model.ConfigFileChangeEvent 配置变更事件
func (s *Source) onChange(event model.ConfigFileChangeEvent) {
	if event.ChangeType == model.Deleted {
		return
	}

	s.notify(conv(event.ConfigFileMetadata.GetFileName(), event.NewValue))
}

// buildClient 构建Polaris SDK上下文
// 设置服务器地址、通信协议与请求超时时间，并基于配置对象构建SDK上下文
// @return @1 api.SDKContext SDK上下文
// @return @2 error 错误信息
func (s *Source) buildClient() (api.SDKContext, error) {
	cfg := api.NewConfiguration()

	cfg.GetGlobal().GetServerConnector().SetAddresses(s.opts.urls)
	cfg.GetGlobal().GetServerConnector().SetProtocol(s.opts.protocol)
	cfg.GetGlobal().GetServerConnector().SetMessageTimeout(s.opts.timeout)
	cfg.GetGlobal().GetAPI().SetTimeout(s.opts.timeout)

	return api.InitContextByConfig(cfg)
}

// isUnimplementedError 判断是否为服务端未实现接口错误
// 服务端版本较旧时可能未实现UpsertAndPublishConfigFile接口，gRPC层返回Unimplemented状态码；
// 该错误已被SDK包装为model.SDKError且底层cause不可导出，无法通过status.Code直接判断，
// 故只能匹配错误文本中的Unimplemented标识
// @param err error 错误信息
// @return @1 bool 是否为未实现接口错误
func isUnimplementedError(err error) bool {
	return strings.Contains(err.Error(), "Unimplemented")
}

// isDataConflictError 判断是否为数据冲突错误
// 部分版本服务端的UpsertAndPublishConfigFile实现存在缺陷，配置文件已存在时返回Code_DataConflict(409000)，
// 此时退化为创建（已存在则更新）后发布
// @param err error 错误信息
// @return @1 bool 是否为数据冲突错误
func isDataConflictError(err error) bool {
	e, ok := err.(model.SDKError)
	if !ok {
		return false
	}

	return strings.Contains(e.Error(), fmt.Sprintf("server code %d", apimodel.Code_DataConflict))
}

// isExistedResourceError 判断是否为资源已存在错误
// 服务端返回Code_ExistedResource(400201)表示配置文件已存在
// @param err error 错误信息
// @return @1 bool 是否为资源已存在错误
func isExistedResourceError(err error) bool {
	e, ok := err.(model.SDKError)
	if !ok {
		return false
	}

	return strings.Contains(e.Error(), fmt.Sprintf("server code %d", apimodel.Code_ExistedResource))
}

// conv 转换配置
// 将配置文件名称和内容转换为统一的配置结构，文件名称的后缀作为配置格式
// @param file string 配置文件名称
// @param content string 配置内容
// @return @1 *config.Configuration 配置项
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
