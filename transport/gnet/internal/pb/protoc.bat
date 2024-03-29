@echo off

for %%c in (*.proto) do (
		protoc --gofast_out=.. --go-grpc_out=.. %%c
)
