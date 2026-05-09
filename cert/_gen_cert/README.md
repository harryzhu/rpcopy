
# 生成指定域名的证书，用于rpcopy加密传输

## Usage

1） 一个`域名`。 

也可以是任意域名，但需要在 `hosts` 文件中将 `域名` 和 `IP` 解析；

```Bash
# 在/etc/hosts 文件中添加记录
# 或者在域名供应商的管理界面，添加 `A记录` 指向
#

12.34.56.7    your.domain.name

```

2） 修改 `openssl.conf`， 将最后一行 `DNS.1 = files.rpcopy.com` 修改为你自己的域名 `DNS.1 = 你的域名`

```Bash
#
[alt_names]  
DNS.1 = your.domain.name

```

3） 修改 `gen_cert.sh` 中第一行的 `USER_DOMAIN` 值为 `你的域名`， 然后 `运行 gen_cert.sh` 生成证书

```Bash
#
USER_DOMAIN="your.domain.name"

```


4） 带参数 `--with-tls` 启动 `rpcopy server`

```Bash
./rpcopy server --target-dir="/data/backup01/nn01"  --host="your.domain.name" --with-tls

```


5）带参数 `--with-tls` 启动 `rpcopy send` 客户端

```Bash
./rpcopy send --source-dir=/logs/nn01  --host="your.domain.name" --with-tls

```

## 注意

启用 `--with-tls` 之后， 如果客户端没有管理员提供的三个证书，将无法正确连接到服务端



