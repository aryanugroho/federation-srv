package handler

import (
	"github.com/micro/federation-srv/federation"
	proto "github.com/micro/federation-srv/proto/federation"

	"golang.org/x/net/context"
)

type Federation struct{}

func (f *Federation) Config(ctx context.Context, req *proto.ConfigRequest, rsp *proto.ConfigResponse) error {
	config := federation.GetConfig()
	rsp.Config = configToProto(config)
	return nil
}
