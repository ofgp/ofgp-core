package httpsvr

import (
	"net/http"

	"github.com/ofgp/ofgp-core/log"
	"github.com/ofgp/ofgp-core/node"

	"github.com/julienschmidt/httprouter"
	"github.com/rs/cors"
	"github.com/spf13/viper"
)

var (
	httpLogger = log.New(viper.GetString("loglevel"), "httpsvr")
)

func panicHandler(res http.ResponseWriter, req *http.Request, err interface{}) {
	httpLogger.Error("http handler panic", "err", err)
}

//basicAuth
func basicAuth(h httprouter.Handle, requiredUser, requiredPassword string) httprouter.Handle {
	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		// Get the Basic Authentication credentials
		if requiredUser == "" || requiredPassword == "" {
			h(w, r, ps)
			return
		}
		user, password, hasAuth := r.BasicAuth()
		if hasAuth && user == requiredUser && password == requiredPassword {
			// Delegate request to the given handle
			h(w, r, ps)
		} else {
			// Request Basic Authentication otherwise
			w.Header().Set("WWW-Authenticate", "Basic realm=Restricted")
			http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		}
	}
}

// StartHTTP 在一个新的goroutine里面启动一个HTTPserver
func StartHTTP(node *node.BraftNode, user, pwd, endpoint string, allowedOrigins []string) {
	hd := NewHTTPHandler(node)
	c := cors.New(cors.Options{
		AllowedOrigins: allowedOrigins,
		AllowedMethods: []string{http.MethodGet, http.MethodPost},
		MaxAge:         600,
		AllowedHeaders: []string{"*"},
	})

	router := httprouter.New()
	router.GET("/", basicAuth(hd.root, user, pwd))
	router.GET("/getblockheight", basicAuth(hd.GetBlockHeight, user, pwd))
	router.GET("/gettransactionbyscid/:txid", basicAuth(hd.GetTxBySidechainTxId, user, pwd))
	router.GET("/getblockbyheight/:height", basicAuth(hd.getBlockByHeight, user, pwd))
	router.GET("/gettxsbyblockrange", basicAuth(hd.getBlockTxsBySec, user, pwd))
	router.POST("/createblock", basicAuth(hd.createBlock, user, pwd))
	router.POST("/watchedtx", basicAuth(hd.addTx, user, pwd))
	//获取节点数据
	router.GET("/nodes", basicAuth(hd.GetNodes, user, pwd))
	//获取当前区块
	router.GET("/block/current", basicAuth(hd.GetCurrentBlock, user, pwd))
	//根据高度范围查询区块
	router.GET("/blocks", basicAuth(hd.GetBlocksByHeightSec, user, pwd))
	router.GET("/block/height/:height", basicAuth(hd.GetBlockByHeight, user, pwd))
	router.GET("/block/blockID/:blockID", basicAuth(hd.GetBlockByID, user, pwd))
	//根据txid查询交易
	router.GET("/transaciton/:txid", basicAuth(hd.GetTransActionByID, user, pwd))
	//token合约注册相关接口
	router.POST("/chainreg", basicAuth(hd.chainRegister, user, pwd))
	router.GET("/chainreg", basicAuth(hd.getChainRegisterID, user, pwd))
	router.POST("/tokenreg", basicAuth(hd.tokenRegister, user, pwd))
	router.GET("/tokenreg", basicAuth(hd.getTokenRegisterID, user, pwd))
	router.POST("/manualmint", basicAuth(hd.manualMint, user, pwd))
	//for wallet app
	router.GET("/exconfig", basicAuth(hd.getExConfig, user, pwd))
	router.GET("/mint_payload", basicAuth(hd.getMintPayload, user, pwd))
	go http.ListenAndServe(endpoint, c.Handler(router))
}
