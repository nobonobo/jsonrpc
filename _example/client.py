# encoding: utf-8

from jsonrpc_requests import Server
s = Server("http://localhost:8080/jsonrpc")
print('reply:', s.Sample.Add([3,4,5,6]))
