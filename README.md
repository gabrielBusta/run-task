# run-task

Build a binary:
```bash
go build -o run-task ./cmd/run-task
```
This will produce a `run-task` binary in your project directory.

Test it:
```bash
export REPOSITORIES="{\"nss\": \"NSS\", \"nspr\": \"NSPR\"}"
go build -o run-task ./cmd/run-task
./run-task --nss-checkout=./build/src/nss --nspr-checkout=./build/src/nspr --task-cwd build/src -- bash -cx "nss/automation/taskcluster/windows/build_gyp.sh --opt"
```
