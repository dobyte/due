package link

//type Transponder struct {
//	link  *Link
//	chMsg chan *transport.DeliverArgs
//}
//
//func NewTransponder(link *Link) *Transponder {
//	t := &Transponder{
//		link:  link,
//		chMsg: make(chan *transport.DeliverArgs, 2000),
//	}
//
//	go t.run()
//
//	return t
//}
//
//func (t *Transponder) deliver(args *transport.DeliverArgs) {
//	t.chMsg <- args
//}
//
//func (t *Transponder) run() {
//	for {
//		select {
//		case args, ok := <-t.chMsg:
//			if !ok {
//				return
//			}
//
//			_, _ = t.link.doNodeRPC(context.Background(), args.Message.Route, args.UID, func(ctx context.Context, client transport.NodeClient) (bool, interface{}, error) {
//				miss, err := client.Deliver(ctx, args)
//				return miss, nil, err
//			})
//		}
//	}
//}
