# Benzhi 评测镜像说明

本项目是一个纯标准库 Go 命令行工具，用于解释 RE2 正则表达式并在文本上执行单次或全局匹配，输出结构化 JSON，包括语法树、命中位置和捕获组。

## 本地构建、运行和测试

```bash
GOTOOLCHAIN=local go build ./...
GOTOOLCHAIN=local go run . --smoke-test
GOTOOLCHAIN=local go test ./...
GOTOOLCHAIN=local go vet ./...
```

示例：

```bash
GOTOOLCHAIN=local go run . explain --pattern '(?P<year>\\d{4})'
GOTOOLCHAIN=local go run . match --pattern '\\d+' --input 'a1 b22' --global
```

## Benzhi Docker 镜像

`build_benzhi_docker.sh` 固定使用 `benzhi.Dockerfile` 构建评测镜像，参数依次为镜像名和平台，默认值为 `my-project` 与 `linux/amd64`。

```bash
bash ./build_benzhi_docker.sh go-task040-regex:amd64 linux/amd64
bash ./build_benzhi_docker.sh go-task040-regex:arm64 linux/arm64
docker run --rm go-task040-regex:amd64 bash -lc 'go run . --smoke-test'
```

镜像启动后默认进入 Bash，便于在容器内执行构建、测试和自检命令；项目自身的运行时镜像仍由 `Dockerfile` 提供。
