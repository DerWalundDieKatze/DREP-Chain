package main

const (
	ConfTemplate = `
{
	"log" : {
	      "logLevel": 4
	},
	"rpc" : {
	      "IPCEnabled": true,
	      "HTTPEnabled": true,
	      "WSEnabled": false,
	      "WSExposeAll": false,
	      "RESTEnabled": false
	},
	"p2p" : {
	      "MaxPeers": 20,
	      "NoDiscovery": false,
	      "DiscoveryV5": true,
	      "Name": "drepnode",
	      "BootstrapNodes": null,
	      "StaticNodes": [
	      ],
	      "ListenAddr": "0.0.0.0:55555"
	},
	"chain" : {
	      "remoteport": 55556,
	      "rootChain": 0,
	      "chainId": 0,
	      "genesispk": "0x03177b8e4ef31f4f801ce00260db1b04cc501287e828692a404fdbc46c7ad6ff26",
	},
	"accounts" : {
	      "enable": true,
          "password": "123"
	},
	"trace":{
		"enable":false,
	}
}
`
)
