package node

type Engine struct {
	router  *Router
	trigger *Trigger
}

//// 分发处理消息
//func (e *Engine) dispatch() {
//	go func() {
//		for {
//			select {
//			case evt, ok := <-e.trigger.receive():
//				if !ok {
//					return
//				}
//				xcall.Call(func() {
//					e.trigger.handle(evt)
//				})
//			case ctx, ok := <-e.router.receive():
//				if !ok {
//					return
//				}
//				xcall.Call(func() {
//					e.router.handle(ctx)
//				})
//			case handle, ok := <-e.fnChan:
//				if !ok {
//					return
//				}
//				xcall.Call(handle)
//			}
//		}
//	}()
//}
