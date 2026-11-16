package polaris

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/utils/xconv"
	"github.com/polarismesh/polaris-go/api"
	plog "github.com/polarismesh/polaris-go/pkg/log"
	"golang.org/x/sync/singleflight"
)

// name is the name of the registry component.
const name = "polaris"

// maxWeight is the upper limit of a service instance weight.
// The valid range of a polaris service instance weight is [0, 10000].
const maxWeight = 10000

const (
	metaFieldID       = "id"
	metaFieldName     = "name"
	metaFieldKind     = "kind"
	metaFieldAlias    = "alias"
	metaFieldState    = "state"
	metaFieldRoutes   = "routes"
	metaFieldEvents   = "events"
	metaFieldWeight   = "weight"
	metaFieldServices = "services"
	metaFieldEndpoint = "endpoint"
	metaFieldMetadata = "metadata"
)

var _ registry.Registry = &Registry{}

type Registry struct {
	err      error
	opts     *options
	builtin  bool
	provider api.ProviderAPI
	consumer api.ConsumerAPI
	watchers sync.Map
	group    singleflight.Group
	closed   atomic.Bool
}

func NewRegistry(opts ...Option) *Registry {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	o.normalize()

	r := &Registry{}
	r.opts = o

	if o.client == nil {
		o.client, r.err = r.buildClient()
		r.builtin = true
	}

	if r.err == nil {
		r.provider = api.NewProviderAPIByContext(o.client)
		r.consumer = api.NewConsumerAPIByContext(o.client)
	}

	return r
}

// Name returns the name of the service registry and discovery component.
func (r *Registry) Name() string {
	return name
}

// Register registers a service instance.
func (r *Registry) Register(ctx context.Context, ins *registry.ServiceInstance) error {
	if r.err != nil {
		return r.err
	}

	if r.closed.Load() {
		return errors.ErrRegistryClosed
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	host, port, protocol, err := parseEndpoint(ins.Endpoint)
	if err != nil {
		return err
	}

	weight := min(max(ins.Weight, 1), maxWeight)

	req := &api.InstanceRegisterRequest{}
	req.Service = ins.Name
	req.Namespace = r.opts.namespace
	req.Host = host
	req.Port = port
	req.Protocol = &protocol
	req.Weight = &weight
	req.Metadata = make(map[string]string, 11)

	req.Metadata[metaFieldID] = ins.ID
	req.Metadata[metaFieldName] = ins.Name
	req.Metadata[metaFieldKind] = ins.Kind
	req.Metadata[metaFieldAlias] = ins.Alias
	req.Metadata[metaFieldState] = ins.State
	req.Metadata[metaFieldEndpoint] = ins.Endpoint
	req.Metadata[metaFieldWeight] = xconv.String(weight)

	if len(ins.Routes) > 0 {
		routes, err := marshalRoutes(ins.Routes)
		if err != nil {
			return err
		}

		req.Metadata[metaFieldRoutes] = routes
	}

	if len(ins.Events) > 0 {
		if events, err := json.Marshal(ins.Events); err != nil {
			return err
		} else {
			req.Metadata[metaFieldEvents] = xconv.BytesToString(events)
		}
	}

	if len(ins.Services) > 0 {
		if services, err := json.Marshal(ins.Services); err != nil {
			return err
		} else {
			req.Metadata[metaFieldServices] = xconv.BytesToString(services)
		}
	}

	if len(ins.Metadata) > 0 {
		if metadata, err := json.Marshal(ins.Metadata); err != nil {
			return err
		} else {
			req.Metadata[metaFieldMetadata] = xconv.BytesToString(metadata)
		}
	}

	resp, err := r.provider.RegisterInstance(req)
	if err != nil {
		return err
	}

	// Existed reports whether the instance already existed before registration. Because
	// registration is idempotent, Existed is true on a repeated registration (for example when
	// updating the instance state), which is still treated as a successful registration.
	if resp.Existed {
		log.Debugf("instance %s re-registered", ins.ID)
	}

	return nil
}

// Deregister deregisters a service instance.
func (r *Registry) Deregister(ctx context.Context, ins *registry.ServiceInstance) error {
	if r.err != nil {
		return r.err
	}

	if r.closed.Load() {
		return errors.ErrRegistryClosed
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	host, port, _, err := parseEndpoint(ins.Endpoint)
	if err != nil {
		return err
	}

	req := &api.InstanceDeRegisterRequest{}
	req.Service = ins.Name
	req.Namespace = r.opts.namespace
	req.Host = host
	req.Port = port

	if err := r.provider.Deregister(req); err != nil {
		return err
	}

	return nil
}

// Watch watches for changes to the service instances with the same service name.
func (r *Registry) Watch(ctx context.Context, serviceName string) (registry.Watcher, error) {
	if r.err != nil {
		return nil, r.err
	}

	if r.closed.Load() {
		return nil, errors.ErrRegistryClosed
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	mgr, err := r.doBuildWatcherMgr(ctx, serviceName)
	if err != nil {
		return nil, err
	}

	return mgr.fork()
}

// doBuildWatcherMgr builds a service instance watcher manager.
func (r *Registry) doBuildWatcherMgr(_ context.Context, serviceName string) (*watcherMgr, error) {
	if mgr := r.loadWatcherMgr(serviceName); mgr != nil {
		return mgr, nil
	}

	v, err, _ := r.group.Do(serviceName, func() (any, error) {
		if mgr := r.loadWatcherMgr(serviceName); mgr != nil {
			return mgr, nil
		}

		mgr := newWatcherMgr(r, serviceName)
		if err := mgr.init(); err != nil {
			return nil, err
		}

		r.watchers.Store(serviceName, mgr)

		// Prevent storing an already closed watcher manager when Close and Watch run
		// concurrently.
		if r.closed.Load() {
			mgr.stop()
			return nil, errors.ErrRegistryClosed
		}

		return mgr, nil
	})
	if err != nil {
		return nil, err
	}

	return v.(*watcherMgr), nil
}

// loadWatcherMgr loads the service watcher manager.
//
// It returns only a manager that has not stopped, avoiding returning one that has stopped but has
// not yet been removed from the registry.
func (r *Registry) loadWatcherMgr(serviceName string) *watcherMgr {
	if v, ok := r.watchers.Load(serviceName); ok {
		if mgr, ok := v.(*watcherMgr); ok && !mgr.stopped.Load() {
			return mgr
		}
	}

	return nil
}

// Services returns the list of service instances.
func (r *Registry) Services(ctx context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	if r.err != nil {
		return nil, r.err
	}

	if r.closed.Load() {
		return nil, errors.ErrRegistryClosed
	}

	if mgr := r.loadWatcherMgr(serviceName); mgr != nil {
		if services, err := mgr.services(); err != nil {
			log.Warnf("load %s services failed: %v", serviceName, err)
		} else {
			return services, nil
		}
	}

	return r.services(ctx, serviceName)
}

// Close closes the service registry and discovery component.
func (r *Registry) Close() error {
	if r.err != nil {
		return r.err
	}

	if !r.closed.CompareAndSwap(false, true) {
		return nil
	}

	r.watchers.Range(func(key, value any) bool {
		value.(*watcherMgr).stop()
		return true
	})

	if r.builtin {
		r.opts.client.Destroy()
	}

	return nil
}

// services returns the list of service instances.
func (r *Registry) services(_ context.Context, serviceName string) ([]*registry.ServiceInstance, error) {
	req := &api.GetAllInstancesRequest{}
	req.Service = serviceName
	req.Namespace = r.opts.namespace

	resp, err := r.consumer.GetAllInstances(req)
	if err != nil {
		return nil, err
	}

	return parseInstances(resp.GetInstances())
}

// setLoggersDir configures the process-wide Polaris SDK loggers once.
//
// The SDK shares its loggers across all contexts, so the built-in client only takes them over
// while they still use the SDK defaults. Loggers configured by the application are left
// untouched, and the configuration runs at most once per process to avoid rebuilding the file
// loggers and leaking the replaced ones.
var (
	setLoggersDirOnce sync.Once
	setLoggersDirErr  error
)

func setLoggersDir(logDir string) error {
	setLoggersDirOnce.Do(func() {
		if !loggersStillDefault() {
			return
		}

		setLoggersDirErr = api.SetLoggersDir(logDir)
	})

	return setLoggersDirErr
}

// loggersStillDefault reports whether every Polaris logger still uses the SDK default directory
// and log level.
func loggersStillDefault() bool {
	loggers := []plog.Logger{
		plog.GetBaseLogger(),
		plog.GetStatLogger(),
		plog.GetStatReportLogger(),
		plog.GetDetectLogger(),
		plog.GetNetworkLogger(),
		plog.GetCacheLogger(),
		plog.GetEventLogger(),
		plog.GetLosslessLogger(),
		plog.GetRouteLogger(),
		plog.GetAuthLogger(),
		plog.GetRateLimitLogger(),
		plog.GetCircuitBreakerLogger(),
	}

	for _, logger := range loggers {
		if l, ok := logger.(plog.DirLogger); ok && !isDefaultLogDir(l.GetLogDir()) {
			return false
		}

		if currentLogLevel(logger) != plog.DefaultLogLevel {
			return false
		}
	}

	return true
}

// isDefaultLogDir reports whether dir is the SDK default log root or one of its subdirectories.
func isDefaultLogDir(dir string) bool {
	root := filepath.Clean(plog.DefaultLogRotationRootDir)

	cleaned := filepath.Clean(dir)

	return cleaned == root || strings.HasPrefix(cleaned, root+string(filepath.Separator))
}

// currentLogLevel returns the lowest log level enabled by logger.
func currentLogLevel(logger plog.Logger) int {
	for _, level := range []int{plog.TraceLog, plog.DebugLog, plog.InfoLog, plog.WarnLog, plog.ErrorLog, plog.FatalLog} {
		if logger.IsLevelEnabled(level) {
			return level
		}
	}

	return plog.NoneLog
}

// buildClient builds the Polaris SDK context.
func (r *Registry) buildClient() (api.SDKContext, error) {
	if err := setLoggersDir(r.opts.logDir); err != nil {
		return nil, err
	}

	cfg := api.NewConfiguration()

	cfg.GetGlobal().GetServerConnector().SetAddresses(r.opts.urls)
	cfg.GetGlobal().GetServerConnector().SetProtocol(r.opts.protocol)
	cfg.GetGlobal().GetServerConnector().SetMessageTimeout(r.opts.timeout)
	cfg.GetGlobal().GetAPI().SetTimeout(r.opts.timeout)
	cfg.GetConsumer().GetLocalCache().SetPersistDir(r.opts.cacheDir)

	return api.InitContextByConfig(cfg)
}
