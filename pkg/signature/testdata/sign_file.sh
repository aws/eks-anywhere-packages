#!/usr/bin/env bash
yq () {
  local file=$1
  local query=$2
  local params=$3
  docker run --rm -i --entrypoint yq linuxserver/yq ${params} -rc "${query}" <${file}
}
file=$1 
alwaysexcludes='.metadata.annotations."eksa.aws.com/signature"'
has_excludes=$(yq "${file}" '.metadata.annotations."eksa.aws.com/excludes"')

excludes=""
if [ "${has_excludes}" != "null" ]; then
    excludes=$(yq "${file}" '.metadata.annotations."eksa.aws.com/excludes"' | base64 -d | cat <(echo ${alwaysexcludes}) - | paste -sd "," -) 
fi
fixed=$(yq ${file} \
    "del(${alwaysexcludes}$([ ! -z ${excludes} ] && echo , ${excludes})) | walk( if type == \"object\" then with_entries(select(.value != \"\" and .value != null and .value != [])) else . end)" "--indentless-lists -Y -S")
digest=$(openssl dgst -sha256 -binary <<<"${fixed}")
signature=$(openssl pkeyutl -inkey pkg/signature/testdata/private.ec.key -sign -in <(echo "${digest}") | base64 -w0)
yq "${file}" ".metadata.annotations.\"eksa.aws.com/signature\" = \"${signature}\"" -Y > "${file}.signed"
echo -n "${digest}" | base64 -w0 > ${file}.digest
