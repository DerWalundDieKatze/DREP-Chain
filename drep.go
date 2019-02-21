package main

import (
	"fmt"
	"github.com/drep-project/drep-chain/log"
	"reflect"

	accountService "github.com/drep-project/drep-chain/accounts/service"
	"github.com/drep-project/drep-chain/app"
	chainService "github.com/drep-project/drep-chain/chain/service"
	consensusService "github.com/drep-project/drep-chain/consensus/service"
	"github.com/drep-project/drep-chain/database"
	p2pService "github.com/drep-project/drep-chain/network/service"
	rpcService "github.com/drep-project/drep-chain/rpc/service"
	cliService "github.com/drep-project/drep-chain/drepclient/service"
)

func main() {
	drepApp := app.NewApp()
	err := drepApp.AddServiceType(
		reflect.TypeOf(database.DatabaseService{}),
		reflect.TypeOf(rpcService.RpcService{}),
		reflect.TypeOf(log.LogService{}),
		reflect.TypeOf(p2pService.P2pService{}),

		reflect.TypeOf(chainService.ChainService{}),
		reflect.TypeOf(accountService.AccountService{}),
		reflect.TypeOf(consensusService.ConsensusService{}),
		reflect.TypeOf(cliService.CliService{}),
	)
	if err != nil {
		fmt.Println(err)
		return
	}

	drepApp.Name = "drep"
	drepApp.Author = "Drep-Project"
	drepApp.Email = ""
	drepApp.Version = "0.1"
	drepApp.HideVersion = true
	drepApp.Copyright = "Copyright 2018 - now The drep Authors"

	if err := drepApp.Run(); err != nil {
		fmt.Println(err)
	}
	return
}