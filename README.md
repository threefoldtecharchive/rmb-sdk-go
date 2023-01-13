[![Go Documentation](https://godocs.io/github.com/threefoldtech/rmb-sdk-go?status.svg)](https://godocs.io/github.com/threefoldtech/rmb-sdk-go)

# Introduction
This is a `GO` sdk that can be used to build both **services**, and **clients**
that can talk over the `rmb`.

[RMB](https://github.com/threefoldtech/rmb-rs) is a message bus that enable secure
and reliable `RPC` calls across the globe.

`RMB` itself does not implement an RPC protocol, but just the secure and reliable messaging
hence it's up to server and client to implement their own data format.

## Pre Requirements
You need to run an `rmb-peer` locally. The `peer` instance establishes a connection to the `rmb-relay` to and works
as a gateway for all services and client behind it.

In another terminal run
```bash
rmb-peer -m "<mnemonics>"
```
> Can be added to the system service with systemd so it can be running all the time

> run `rmb-peer -h` to customize the peer, including which relay and which tfchain to connect to.

# Example
Please check the example directory for code examples
- [server](examples/server/main.go)
- [client](examples/client/main.go)

### Direct Client
There is a `direct` client that does not require `rmb-peer` and can connect directly to the rmb `relay`. This client is defined under
[direct](direct)
