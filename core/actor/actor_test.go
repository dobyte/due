package actor_test

import (
	"github.com/dobyte/due/v2/core/actor"
	"testing"
)

type Table struct {
}

func NewTable(actor actor.Actor) actor.Processor {
	return &Table{}
}

// Kind 类型
func (t *Table) Kind() string {
	return "table"
}

// Init 初始化回调
func (t *Table) Init() {

}

// Start 启动回调
func (t *Table) Start() {

}

// Destroy 销毁回调
func (t *Table) Destroy() {

}

func TestSpawn(t *testing.T) {
	actor.Spawn(NewTable)
}
