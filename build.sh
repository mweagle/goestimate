#! /bin/zsh -ex

go generate ./...

rm -fv ./examples/*.d2
rm -fv ./examples/*.dot

for eachExample in ./examples/*.json; do
    go run main.go --input=$eachExample
done

go build .