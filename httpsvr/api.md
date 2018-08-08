# 返回数据格式

```
{
	code:200,
	msg:"ok",
	data:{},
}
```
code|说明|
:----|:----
200|正常返回
501|参数错误
502|系统错误
# 区块和交易

交易字段说明
字段|类型|说明
:----|:----|:----
from_tx_hash|string|转出链交易id
to_tx_hash|string|转入链交易id
dgw_hash|string|网关交易id
from|string|转出链类型btc/bch/eth
to|string|转入链类型btc/bch/eth
time|int64|交易时间unix时间戳
block|string|所在区块id
amount|int64|交易值
to_addrs|[]string|转入链地址
from_fee|int64|转出链矿工费
dgw_fee|int64|网关矿工费（暂无）
to_fee|int64|转入链矿工费 (暂无)

区块字段说明
字段|类型|说明
:----|:----|:----
height|int64|高度
id|string|区块id
pre_id|string|前一区块id
tx_cnt|int|包含的交易数量
txs|[]transaction|包含的交易
time|int64|区块创建时间 unix时间戳
size|int|区块大小
created_used|int64|距上一个区块的时间
miner|string|创建区块的server

1.查询当前区块

GET /block/current

res

```
{
"code":200,
"msg":ok,
"data":{
	"height": 1,
	"id": "2cf26b3e040651f2248e5e39c5ce4da59ffb2b55a1ba1c2a74eba23513a455de",
	"pre_id": "3f9b948824850d63fddedbf1ac0e49d811ef67bed72c60453ce4f5b793c04c72",
	"tx_cnt": 1,
	"txs": [{
		"from_tx_hash": "88185d128d9922e0e6bcd32b07b6c7f20f27968eab447a1d8d1cdf250f79f7d3",
		"to_tx_hash": "88185d128d9922e0e6bcd32b07b6c7f20f27968eab447a1d8d1cdf250f79f7d3",
		"dgw_hash": "c3fceae1466ab03bca793cbb3455c3c2b9920042b78d231e12b0d9be97a71493",
		"from": "bch",
		"to": "eth",
		"time": 1530088946,
		"block": "2cf26b3e040651f2248e5e39c5ce4da59ffb2b55a1ba1c2a74eba23513a455de",
		"amount": 10,
		"to_addrs": ["88185d128d9922e0e6bcd32b07b6c7f20f27968eab447a1d8d1cdf250f79f7d3"],
		"from_fee": 1,
		"dgw_fee": 0,
		"to_fee": 0
	}],
	"time": 1530086629,
	"size": 364,
	"created_used": 10,
	"miner": "server0"
}
}
```
2.根据高度区间查询区块(>=start < end)

GET /blocks?start={start}&end={end}

res

```
{
"code":200,
"msg":ok,
"data":[
    {
	"height": 1,
	"id": "2cf26b3e040651f2248e5e39c5ce4da59ffb2b55a1ba1c2a74eba23513a455de",
	"pre_id": "3f9b948824850d63fddedbf1ac0e49d811ef67bed72c60453ce4f5b793c04c72",
	"tx_cnt": 1,
	"Txs": [{
		"from_tx_hash": "88185d128d9922e0e6bcd32b07b6c7f20f27968eab447a1d8d1cdf250f79f7d3",
		"to_tx_hash": "88185d128d9922e0e6bcd32b07b6c7f20f27968eab447a1d8d1cdf250f79f7d3",
		"dgw_hash": "c3fceae1466ab03bca793cbb3455c3c2b9920042b78d231e12b0d9be97a71493",
		"from": "bch",
		"to": "eth",
		"time": 1530088946,
		"block": "2cf26b3e040651f2248e5e39c5ce4da59ffb2b55a1ba1c2a74eba23513a455de",
		"amount": 10,
		"to_addrs": ["88185d128d9922e0e6bcd32b07b6c7f20f27968eab447a1d8d1cdf250f79f7d3"],
		"from_fee": 1,
		"dgw_fee": 0,
		"to_fee": 0
	}],
	"time": 1530086629,
	"size": 364,
	"created_used": 10,
	"miner": "server0"
}
]
}
```
3、根据高度查询区块

GET /block/height/1

res

```
{
"code":200,
"msg":"ok",
"data":{
	"height": 1,
	"id": "2cf26b3e040651f2248e5e39c5ce4da59ffb2b55a1ba1c2a74eba23513a455de",
	"pre_id": "3f9b948824850d63fddedbf1ac0e49d811ef67bed72c60453ce4f5b793c04c72",
	"tx_cnt": 1,
	"Txs": [{
		"from_tx_hash": "88185d128d9922e0e6bcd32b07b6c7f20f27968eab447a1d8d1cdf250f79f7d3",
		"to_tx_hash": "88185d128d9922e0e6bcd32b07b6c7f20f27968eab447a1d8d1cdf250f79f7d3",
		"dgw_hash": "c3fceae1466ab03bca793cbb3455c3c2b9920042b78d231e12b0d9be97a71493",
		"from": "bch",
		"to": "eth",
		"time": 1530088946,
		"block": "2cf26b3e040651f2248e5e39c5ce4da59ffb2b55a1ba1c2a74eba23513a455de",
		"amount": 10,
		"to_addrs": ["88185d128d9922e0e6bcd32b07b6c7f20f27968eab447a1d8d1cdf250f79f7d3"],
		"from_fee": 1,
		"dgw_fee": 0,
		"to_fee": 0
	}],
	"time": 1530086629,
	"size": 364,
	"created_used": 10,
	"miner": "server0"
}
}
```

4、根据blockID查询block

GET /block/blockID/{blockID}

res

```
{
"code":200,
"msg":"ok",
"data":{
	"height": 1,
	"id": "2cf26b3e040651f2248e5e39c5ce4da59ffb2b55a1ba1c2a74eba23513a455de",
	"pre_id": "3f9b948824850d63fddedbf1ac0e49d811ef67bed72c60453ce4f5b793c04c72",
	"tx_cnt": 1,
	"Txs": [{
		"from_tx_hash": "88185d128d9922e0e6bcd32b07b6c7f20f27968eab447a1d8d1cdf250f79f7d3",
		"to_tx_hash": "88185d128d9922e0e6bcd32b07b6c7f20f27968eab447a1d8d1cdf250f79f7d3",
		"dgw_hash": "c3fceae1466ab03bca793cbb3455c3c2b9920042b78d231e12b0d9be97a71493",
		"from": "bch",
		"to": "eth",
		"time": 1530088946,
		"block": "2cf26b3e040651f2248e5e39c5ce4da59ffb2b55a1ba1c2a74eba23513a455de",
		"amount": 10,
		"to_addrs": ["88185d128d9922e0e6bcd32b07b6c7f20f27968eab447a1d8d1cdf250f79f7d3"],
		"from_fee": 1,
		"dgw_fee": 0,
		"to_fee": 0
	}],
	"time": 1530086629,
	"size": 364,
	"created_used": 10,
	"miner": "server0"
}
}
```

4.根据交易id查询交易

GET /transaction/{txid}

res
```
{
"code":200,
"msg":"ok",
"data":{
	"from_tx_hash": "88185d128d9922e0e6bcd32b07b6c7f20f27968eab447a1d8d1cdf250f79f7d3",
	"to_tx_hash": "88185d128d9922e0e6bcd32b07b6c7f20f27968eab447a1d8d1cdf250f79f7d3",
	"dgw_hash": "c3fceae1466ab03bca793cbb3455c3c2b9920042b78d231e12b0d9be97a71493",
	"from": "bch",
	"to": "eth",
	"time": 1530088946,
	"block": "0c5efde1309925920e67454b3cba41acbe2245f603ac2e88d2a53a28dc6ffffd",
	"amount": 10,
	"to_addrs": ["88185d128d9922e0e6bcd32b07b6c7f20f27968eab447a1d8d1cdf250f79f7d3"],
	"from_fee": 1,
	"dgw_fee": 0,
	"to_fee": 0
}
}
```

# 节点

字段说明
字段|类型|说明
:----|:----|:----
ip|string|
host_name|string|
is_leader|bool|是否leader
is_online|bool|是否在线
fired_cnt|int32|被替换掉leader的次数从启动开始
eth_height|int64|监听到的eth高度
bch_height|int64|监听到的bch高度
btc_height|int64|监听到的btc高度
1.查询节点

GET /nodes

res

```
[{
	"ip": "127.0.0.1",
	"host_name": "server0",
	"is_leader": true,
	"is_online": true,
	"fired_cnt": 0,
	"eth_height": 3,
	"bch_height": 2,
	"btc_height": 1
}]
```
