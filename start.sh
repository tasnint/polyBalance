#!/bin/bash

echo "Starting 10 backend servers..."
go run ./cmd/backend -port 8081 -name backend-1 &
go run ./cmd/backend -port 8082 -name backend-2 &
go run ./cmd/backend -port 8083 -name backend-3 &
go run ./cmd/backend -port 8084 -name backend-4 &
go run ./cmd/backend -port 8085 -name backend-5 &
go run ./cmd/backend -port 8086 -name backend-6 &
go run ./cmd/backend -port 8087 -name backend-7 &
go run ./cmd/backend -port 8088 -name backend-8 &
go run ./cmd/backend -port 8089 -name backend-9 &
go run ./cmd/backend -port 8090 -name backend-10 &

sleep 3

echo "Starting PolyBalance load balancer..."
export LB_BACKENDS="http://localhost:8081,http://localhost:8082,http://localhost:8083,http://localhost:8084,http://localhost:8085,http://localhost:8086,http://localhost:8087,http://localhost:8088,http://localhost:8089,http://localhost:8090"
go run ./cmd
