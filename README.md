# rediscopy

通过抓取网卡数据实现redis流量的简单复制。它类似`TCPCopy`,但是在应用层对redis协议进行了解析，并将redis数据转发到另外的服务器上。

它可以实时抓取，也可以读取tcpdump等工具抓取的离线包。

比如:

```
go run . -f "proxy.pcap" -addr "127.0.0.1:9528" -d
```

