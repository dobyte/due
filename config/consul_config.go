package config

import (
	"container/list"
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"strings"
	"time"

	capi "github.com/hashicorp/consul/api"
	cwatcher "github.com/pteich/consul-kv-watcher"
	"github.com/sirupsen/logrus"
)

var kv *capi.KV

const consul_key_prefix = "config/"
const local_key_prefix = "config."

const defaultAddrKey = "config.registry.consul.addr"
const defaultAddr = "http://127.0.0.1:8500"

var cacheMap *Map[[]byte]

func InitConsulConfig(ctx context.Context) {
	consulUrl := Get(defaultAddrKey, defaultAddr).String()
	// Get a new client
	config := capi.DefaultConfig()
	config.Address = consulUrl
	client, err := capi.NewClient(config)
	if err != nil {
		panic(err)
	}

	cacheMap = NewMap[[]byte]()

	// Get a handle to the KV API
	kv = client.KV()

	// init cacheMap
	pairs, _, err := kv.List(consul_key_prefix, nil)
	if err != nil {
		panic(err)
	}

	tempMap := make(map[string][]byte)
	for _, pair := range pairs {
		key := strings.Replace(pair.Key, consul_key_prefix, "", 1)
		tempMap[key] = pair.Value
		Set(local_key_prefix+key, pair.Value)
	}
	cacheMap.ResetAll(tempMap)

	watcher := cwatcher.New(client, 10*time.Second, 0)
	watchChan, _ := watcher.WatchTree(ctx, consul_key_prefix)
	go watchChange(ctx, watchChan)

	logrus.Info("consul config init")
}

func watchChange(ctx context.Context, watchChan <-chan capi.KVPairs) {
	for {
		select {
		case <-ctx.Done():
			return
		case pairs, ok := <-watchChan:
			if !ok {
				return
			}
			if pairs == nil {
				return
			}
			logrus.Debug("consul change watched")
			tempMap := make(map[string][]byte)
			for _, pair := range pairs {
				key := strings.Replace(pair.Key, consul_key_prefix, "", 1)
				tempMap[key] = pair.Value
				Set(local_key_prefix+key, pair.Value)
			}
			changes := getConfigChange(cacheMap.mp, tempMap)
			cacheMap.ResetAll(tempMap)
			// push change
			cacheMap.pushChangeEvent(changes)
		}
	}
}

func PutConsulConfig(key string, value []byte) error {
	// PUT a new KV pair
	p := &capi.KVPair{Key: consul_key_prefix + key, Value: value}
	_, err := kv.Put(p, nil)
	return err
}

func PutConsulKV(key, value string) error {
	p := &capi.KVPair{Key: consul_key_prefix + key, Value: []byte(value)}
	_, err := kv.Put(p, nil)
	return err
}

// default http timeout, take care
func GetConsulConfigRemote(key string) ([]byte, error) {
	pair, _, err := kv.Get(consul_key_prefix+key, nil)
	if err != nil {
		return nil, err
	}
	if pair == nil {
		return nil, nil
	}
	return pair.Value, nil
}

func GetConsulConfig(key string) ([]byte, bool) {
	return cacheMap.Get(key)
}

func GetConsulKV(key string) (string, error) {
	val, ok := cacheMap.Get(key)
	if !ok {
		return "", nil
	}
	return string(val), nil
}

func GetStringMapKV(key string) (map[string]string, error) {
	val, ok := cacheMap.Get(key)
	if !ok {
		return nil, nil
	}
	resMap := make(map[string]string)
	err := json.Unmarshal(val, &resMap)
	if err != nil {
		return nil, err
	}
	return resMap, nil
}

func GetMapKV[T any](key string) (map[string]T, error) {
	val, ok := cacheMap.Get(key)
	if !ok {
		return nil, nil
	}
	resMap := make(map[string]T)
	err := json.Unmarshal(val, &resMap)
	if err != nil {
		return nil, err
	}
	return resMap, nil
}

func GetIntKV(key string, defaultVal int) int {
	valStr, err := GetConsulKV(key)
	if err != nil {
		logrus.Errorf("get %s value failed: %+v", key, err)
		return defaultVal
	}
	if valStr == "" {
		return defaultVal
	}
	result, err := strconv.Atoi(valStr)
	if err != nil {
		logrus.Errorf("%s parse to int failed: %+v", key, err)
		return defaultVal
	}
	return result
}

func GetCacheVersion(defaultVal int) int {
	return GetIntKV("cache_version", defaultVal)
}

func GetBoolKV(key string, defaultVal bool) bool {
	valStr, err := GetConsulKV(key)
	if err != nil {
		logrus.Errorf("get %s value failed: %+v", key, err)
		return defaultVal
	}
	if valStr == "" {
		return defaultVal
	}
	result, err := strconv.ParseBool(valStr)
	if err != nil {
		logrus.Errorf("%s parse to bool failed: %+v", key, err)
		return defaultVal
	}
	return result
}

func GetStringKV(key string, defaultVal string) string {
	valStr, err := GetConsulKV(key)
	if err != nil {
		logrus.Errorf("get %s value failed: %+v", key, err)
		return defaultVal
	}
	if valStr == "" {
		return defaultVal
	}
	return valStr
}

func GetStructKV[T any](key string) (*T, error) {
	val, ok := cacheMap.Get(key)
	if !ok {
		return nil, nil
	}
	res := new(T)
	err := json.Unmarshal(val, res)
	if err != nil {
		logrus.Errorf("key:%s, unmarshal struct failed, %+v", key, err)
		return nil, err
	}
	return res, nil
}

func GetDurationKV(key string, defaultVal time.Duration) time.Duration {
	valStr, err := GetConsulKV(key)
	if err != nil {
		logrus.Errorf("get %s value failed: %+v", key, err)
		return defaultVal
	}
	if valStr == "" {
		return defaultVal
	}
	v, err := time.ParseDuration(valStr)
	if err != nil {
		logrus.Errorf("key:%s parse time failed: %+v", key, err)
		return defaultVal
	}
	return v
}

func GetStringSliceKV(key string) ([]string, error) {
	val, ok := cacheMap.Get(key)
	if !ok {
		return nil, nil
	}
	var res []string
	err := json.Unmarshal(val, &res)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func AddChangeListener(listener ChangeListener) {
	cacheMap.AddChangeListener(listener)
}
func RemoveChangeListener(listener ChangeListener) {
	cacheMap.RemoveChangeListener(listener)
}
func GetChangeListeners() *list.List {
	return cacheMap.GetChangeListeners()
}

func getConfigChange(oldMap map[string][]byte, newMap map[string][]byte) map[string]*ConfigChange {
	// get old keys
	mp := map[string]struct{}{}
	for k := range oldMap {
		mp[k] = struct{}{}
	}

	changes := make(map[string]*ConfigChange)

	// update new
	// keys
	for key, value := range newMap {
		// key state insert or update
		// insert
		if _, ok := mp[key]; !ok {
			changes[key] = createAddConfigChange(value)
		} else {
			// update
			oldValue, _ := oldMap[key]
			if !reflect.DeepEqual(oldValue, value) {
				changes[key] = createModifyConfigChange(oldValue, value)
			}
		}
		delete(mp, key)
	}

	// remove del keys
	for key := range mp {
		// get old value and del
		oldValue := oldMap[key]
		changes[key] = createDeletedConfigChange(oldValue)
	}

	return changes
}
