package apollo

import (
	"fmt"
	"github.com/apolloconfig/agollo/v4"
	apolloConfig "github.com/apolloconfig/agollo/v4/env/config"
)

func InitApolloClient(appId string, namespace string, ip string, port int) agollo.Client {
	config := &apolloConfig.AppConfig{
		AppID:         appId,
		IP:            fmt.Sprintf("%v:%v", ip, port),
		Cluster:       "dev",
		NamespaceName: namespace,
	}

	client, _ := agollo.StartWithConfig(func() (*apolloConfig.AppConfig, error) {
		return config, nil
	})

	return client
}
