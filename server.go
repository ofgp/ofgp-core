package main

import (
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"
	"time"

	sg "github.com/ofgp/ofgp-core/util/signal"

	"github.com/ofgp/ofgp-core/cluster"
	"github.com/ofgp/ofgp-core/httpsvr"
	"github.com/ofgp/ofgp-core/node"
	"github.com/ofgp/ofgp-core/util"

	"github.com/ofgp/ofgp-core/accuser"

	"github.com/rcrowley/go-metrics"
	"github.com/spf13/viper"
	"github.com/vrischmann/go-metrics-influxdb"
	"gopkg.in/urfave/cli.v1"
)

var (
	app       = util.NewApp()
	signalSet = sg.NewSignalSet()
)

func init() {
	app.Action = run
	app.HideVersion = true
	app.Copyright = "Copyright"
	app.Commands = []cli.Command{}

	//app.Flags = append(app.Flags, util.P2PPortFlag)
	//app.Flags = append(app.Flags, util.DBPathFlag)
	//app.Flags = append(app.Flags, util.HTTPPortFlag)
	app.Flags = append(app.Flags, util.ConfigFileFlag)
	app.Flags = append(app.Flags, util.CPUProfileFlag)
	app.Flags = append(app.Flags, util.MemProfileFlag)
	//app.Flags = append(app.Flags, util.BchHeightFlag)
	for _, flag := range util.Flags {
		app.Flags = append(app.Flags, flag)
	}
}

func baseMetrics() {
	interval := viper.GetDuration("METRICS.interval")
	r := metrics.NewRegistry()

	metrics.RegisterDebugGCStats(r)
	go metrics.CaptureDebugGCStats(r, interval)
	metrics.RegisterRuntimeMemStats(r)
	go metrics.CaptureRuntimeMemStats(r, interval)

	g := metrics.NewGauge()
	r.Register("numgoroutine", g)
	go func() {
		for {
			g.Update(int64(runtime.NumGoroutine()))
			time.Sleep(interval)
		}
	}()
	go influxdb.InfluxDB(r, 10e9, viper.GetString("METRICS.influxdb_uri"),
		viper.GetString("METRICS.db"), viper.GetString("METRICS.user"),
		viper.GetString("METRICS.password"))
}

func run(ctx *cli.Context) {
	configFile := util.GetConfigFile(ctx)
	viper.SetConfigFile(configFile)
	viper.ReadInConfig()
	util.ReadConfigToViper(ctx)

	// 如果需要做性能检测
	cpuProfile := util.GetCPUProfile(ctx)
	if len(cpuProfile) > 0 {
		f, err := os.Create(cpuProfile)
		if err != nil {
			panic("create cpu profile failed")
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}
	memProfile := util.GetMemProfile(ctx)
	if len(memProfile) > 0 {
		f, err := os.Create(memProfile)
		if err != nil {
			panic("create mem profile failed")
		}
		defer pprof.WriteHeapProfile(f)
	}

	if len(viper.GetString("DGW.pprof_host")) > 0 {
		go func() {
			log.Println(http.ListenAndServe(viper.GetString("DGW.pprof_host"), nil))
			// log.Println(http.ListenAndServe(":8060", nil))
		}()
	}

	nodeId := viper.GetInt32("DGW.local_id")
	startMode := viper.GetInt32("DGW.start_mode")

	//设置btc bch 确认块
	node.BtcConfirms = viper.GetInt("DGW.btc_confirms")
	node.BchConfirms = viper.GetInt("DGW.bch_confirms")
	node.EthConfirms = viper.GetInt("DGW.eth_confirms")
	node.EOSConfirms = viper.GetInt("DGW.eos_confirms")
	//交易处理超时时间
	node.ConfirmTolerance = viper.GetDuration("DGW.confirm_tolerance")
	//交易链上check并发数
	node.CheckOnChainCur = viper.GetInt("DGW.check_onchain_cur")
	//交易链上check 周期
	node.CheckOnChainInterval = viper.GetDuration("DGW.check_onchain_interval")

	//设置发起accuse 的间隔
	accuser.AccuseInterval = viper.GetInt64("DGW.accuse_interval")

	var joinMsg *node.JoinMsg
	if startMode == cluster.ModeNormal {
		cluster.Init()
	} else {
		joinMsg = node.InitJoin(startMode)
		nodeId = joinMsg.LocalID
	}

	httpPort := viper.GetInt("DGW.local_http_port")
	cros := []string{}
	if nodeId < 0 || int(nodeId) >= len(cluster.NodeList) {
		panic(fmt.Sprintf("Invalid nodeid %d cluster size %d", nodeId, len(cluster.NodeList)))
	}

	var multiSigs []cluster.MultiSigInfo
	if joinMsg != nil && len(joinMsg.MultiSigInfos) > 0 {
		multiSigs = joinMsg.MultiSigInfos
	}
	_, node := node.RunNew(nodeId, multiSigs)

	user := viper.GetString("DGW.local_http_user")
	pwd := viper.GetString("DGW.local_http_pwd")
	httpsvr.StartHTTP(node, user, pwd, fmt.Sprintf(":%d", httpPort), cros)

	needMetrics := viper.GetBool("METRICS.need_metrics")
	if needMetrics {
		baseMetrics()
	}

	// 添加需要捕获的信号
	if startMode != cluster.ModeWatch && startMode != cluster.ModeTest {
		signalSet.Register(syscall.SIGINT, node.LeaveCluster)
	} else {
		// 观察节点只用自己退出就可以了，不用发LeaveRequest
		signalSet.Register(syscall.SIGINT, node.Stop)
	}
	sigChan := make(chan os.Signal)
	signal.Notify(sigChan, syscall.SIGINT)
	sig := <-sigChan
	fmt.Printf("receive signal %v\n", sig)
	signalSet.Handle(sig)
}

func main() {
	if err := app.Run(os.Args); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
