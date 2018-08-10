# ofgp-core
实现网关的底层逻辑。具体描述见Wiki
# Install
下载源码：

`go get github.com/ofgp/ofgp-core`
或者
`git clone https://github.com/ofgp/ofgp-core.git`

运行：
`go run server.go --config config.toml`

# 配置文件说明

```
# 全节点的类型，生成BCH多签地址的时候会有不同的前缀
net_param = "regtest"
# 日志级别，分别是debug, info, warn, error, critical
loglevel = "info"

[BTC]
# 全节点相关配置
rpc_server = "127.0.0.1:8447"
rpc_user = ""
rpc_password = ""
confirm_block_num = 6
coinbase_confirm_block_num = 100

[BCH]
rpc_server = "127.0.0.1:8445"
rpc_user = ""
rpc_password = ""
confirm_block_num = 6
coinbase_confirm_block_num = 100

[LEVELDB]
# 分别是btc, bch, eth全节点的watcher各自保存数据的目录
btc_db_path = "/data/leveldb_data/btc_tx_db"
bch_db_path = "/data/leveldb_data/bch_tx_db"
ew_nonce_db_path = "/data/leveldb_data/ew_tx_db"

# keystore server的配置。网关节点的信息加签是keystore server来做的，网关节点本身不保存自己的私钥。
[KEYSTORE]
# keystore server的地址
url = "http://127.0.0.1:8976"
# 本节点访问ks的标识
local_pubkey_hash = "3722834BCB13F7308C28907B69A99DB462F39036"
# 整个集群的信息，包括集群节点的个数以及每个节点的公钥
count = 4
key_0 = "04A2E82BE35D90D954E15CC5865E2F8AC22FD2DDBD4750F4BFC7596363A3451D1B75F4A8BAD28CF48F63595349DBC141D6D6E21F4FEB65BDC5E1A8382A2775E787"
key_1 = "049FD6230E3BADBBC7BA190E10B2FC5C3D8EA9B758A43E98AB2C8F83C826AE7EABEA6D88880BC606FA595CD8DD17FC7784B3E55D8EE0705045119545A803215B80"
key_2 = "044667E5B36F387C4D8D955C33FC271F46D791FD3433C0B2F517375BBD9AAE6B8C2392229537B109AC8EADCCE104AEAA64DB2D90BEF9008A09F8563CDB05FFB60B"
key_3 = "0402580DDB61E10132A58BA6E18F78DC62F522653C08AB9E0DFED8F08DF380D35EF29DA7A8DB1D72AB67851FA3BBE5C71DCDB82F3A7A8785A0A1E7B1F2B042939E"
# ks的访问凭证
service_id = "0daf7126-ebbb-4b2d-86f8-a480c1fd45a8"
keystore_private_key = "C51C9CB7A7EC9D12BB37B3700856690719A44056B750AB03A21247A4903BF3CB"

# 网关节点的一些相关配置
[DGW]
# 集群节点个数
count = 4
# 本节点的ID，从0开始+1递增
local_id = 0
# 本节点的p2p网络的端口
local_p2p_port = 10000
# 本节点的http rpc接口的端口
local_http_port = 8280
# 集群所有节点的p2p网络的地址以及相应的状态，正常状态为true
host_0 = "127.0.0.1:10000"
status_0 = true
host_1 = "127.0.0.1:10001"
status_1 = true
host_2 = "127.0.0.1:10002"
status_2 = true
host_3 = "127.0.0.1:10003"
status_3 = true

# 这两项配置用于配置允许新加入的节点的地址和其相应的公钥
new_node_host = "127.0.0.1:10001"
new_node_pubkey = "049FD6230E3BADBBC7BA190E10B2FC5C3D8EA9B758A43E98AB2C8F83C826AE7EABEA6D88880BC606FA595CD8DD17FC7784B3E55D8EE0705045119545A803215B80"

# 初始监听的全节点的高度，如果watcher保存数据的目录没有被删除，不用每次重新设置，watcher会自动保存当前监听到的高度
btc_height = 70209
bch_height = 83667
eth_height = 478779

# 网关节点本身区块以及交易的保存路径
dbpath = "/data/leveldb_data/node"
# eth的全节点地址
eth_client_url = "ws://127.0.0.1:8830"

# start_mode指定节点的启动方式，1是正常启动，利用配置文件里面的节点信息建立网关集群，2是加入已有网关集群, 3是以观察节点的方式启动
start_mode = 1
# init_node_host仅在mode!=1时才有意义，作为新节点的引导节点
init_node_host = "127.0.0.1:10000"
local_host = "127.0.0.1:10001"
local_pubkey = "049FD6230E3BADBBC7BA190E10B2FC5C3D8EA9B758A43E98AB2C8F83C826AE7EABEA6D88880BC606FA595CD8DD17FC7784B3E55D8EE0705045119545A803215B80"

[METRICS]
# 是否需要上报，会上报运行时的内存和goroutine等信息到influxdb，自行配置grafana做展示，用于查看节点运行时占用机器资源的情况
need_metrics = false
# 每隔多久上报一次
interval = 100e6
influxdb_uri = "http://127.0.0.1:8086"
db = "dgw_test"
user = ""
password = ""

[ETHWATCHER]
# 网关合约地址
vote_contract = "0xxxx"
```

