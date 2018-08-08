install
# 设置GOPATH
export GOPATH=path

# 下载依赖
go get -u github.com/btcsuite/btcd

go get -u github.com/btcsuite/btcutil

go get -u github.com/ethereum/go-ethereum
go get -u github.com/spf13/viper
# clone代码，将watcher 移动到GOPATH src 目录

mkdir -p $GOPATH/src

cd $GOPATH/src

git clone ssh://git@47.98.167.79:10022/backend/dgateway.git

git clone ssh://git@47.98.167.79:10022/backend/bitcoinWatcher.git

git clone ssh://git@47.98.167.79:10022/backend/ethWatcher.git

#glide下载依赖

下载之前在glide.yaml ignore 添加：

```
- github.com/btcsuite/btcd

- github.com/btcsuite/btcutil

- github.com/ethereum/go-ethereum
- github.com/spf13/viper
```
cd $GOPATH/src/dgateway

glide install

cd $GOPATH/src/bitcoinWatcher

glide install

cd $GOPATH/src/etchWatcher

glide install



