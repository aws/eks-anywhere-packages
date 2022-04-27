#!/usr/bin/env bash
yq () {
  local file=$1
  local query=$2
  local params=$3
  docker run --rm -i --entrypoint yq linuxserver/yq ${params} -rc "${query}" <${file}
}
file=$1 
tmpfile=$(mktemp ${file}.digest.XXXXXX)
alwaysexcludes='.metadata.annotations."eksa.aws.com/signature"'
has_excludes=$(yq "${file}" '.metadata.annotations."eksa.aws.com/excludes"')

excludes=""
if [ "${has_excludes}" != "null" ]; then
    excludes=$(yq "${file}" '.metadata.annotations."eksa.aws.com/excludes"' | base64 -d | cat <(echo ${alwaysexcludes}) - | paste -sd "," -) 
fi
yq ${file} "del(${alwaysexcludes}$([ ! -z ${excludes} ] && echo , ${excludes})) | walk( if type == \"object\" then with_entries(select(.value != \"\" and .value != null and .value != [])) else . end)" "--indentless-lists -Y -S" | openssl dgst -sha256 -binary >${tmpfile}
signature=$(openssl pkeyutl -inkey pkg/signature/testdata/private.ec.key -sign -in ${tmpfile} | base64 | tr -d '\n')
yq "${file}" ".metadata.annotations.\"eksa.aws.com/signature\" = \"${signature}\"" -Y > "${file}.signed"
cat ${tmpfile} | base64 | tr -d '\n' > ${file}.digest
rm -f ${tmpfile}
