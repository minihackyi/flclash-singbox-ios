//go:build ignore

package main

import (
	"context"
	"fmt"
	"os"
	"time"

	box "github.com/sagernet/sing-box"
	"github.com/sagernet/sing-box/include"
	"github.com/sagernet/sing-box/option"
	singjson "github.com/sagernet/sing/common/json"
)

func main() {
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	registryCtx := box.Context(context.Background(), include.InboundRegistry(), include.OutboundRegistry(), include.EndpointRegistry())
	options, err := singjson.UnmarshalExtendedContext[option.Options](registryCtx, data)
	if err != nil {
		panic(err)
	}
	instance, err := box.New(box.Options{Options: options, Context: registryCtx})
	if err != nil {
		panic(err)
	}
	if err := instance.Start(); err != nil {
		panic(err)
	}
	fmt.Println("== box running, press ctrl-c after tests ==")
	time.Sleep(60 * time.Second)
	instance.Close()
}
