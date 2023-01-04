package feishu_test

import (
	"github.com/dobyte/due/alert/feishu"
	"testing"
)

var alerter *feishu.Alerter

func init() {
	alerter = feishu.NewAlerter(
		feishu.WithWebhook("https://open.feishu.cn/open-apis/bot/v2/hook/231e12c0-97b9-4e81-a920-abc24bda286f"),
		feishu.WithSecret(""),
	)
}

func TestAlert_SendTextMessage(t *testing.T) {
	err := alerter.Alert("Alerter test project, Please ignore this message")
	if err != nil {
		t.Fatal(err)
	}
}
