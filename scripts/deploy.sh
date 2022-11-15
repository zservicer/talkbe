jump_ssh=ymipro
deploy_dir="/services/deploy/servicer"
root_dir="/services/servicer/allinone"
service=allinone

dest=./dest
rm -rf $dest

mkdir $dest

GOARCH=amd64 GOOS=linux go build -ldflags "-s -w" -o $dest/${service} cmd/${service}/main.go
upx --brute $dest/${service}

ssh ${jump_ssh} "cp ${deploy_dir}/${service} ${deploy_dir}/${service}_$(date +%Y%m%d%H%M%S) || true"
scp $dest/${service} ${jump_ssh}:${deploy_dir}/
ssh ${jump_ssh} "cd ${deploy_dir} && sh ../_deploy_v3.sh ${root_dir} ${service}"
