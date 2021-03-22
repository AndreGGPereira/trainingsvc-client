#!/bin/bash

if [[ -e .env ]];then
  export $(egrep -v '^#' .env | xargs)
fi

go run cmd/service/main.go --dbcs="admin:password@tcp(localhost:3306)/ms_training?parseTime=true"