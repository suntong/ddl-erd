templateFile=$GOPATH/src/github.com/go-easygen/easygen/test/commandlineFlag
[ -s $templateFile.tmpl ] || templateFile=/usr/share/gocode/src/github.com/go-easygen/easygen/test/commandlineFlag
[ -s $templateFile.tmpl ] || templateFile=/usr/share/doc/easygen/examples/commandlineFlag
[ -s $templateFile.tmpl ] || {
  echo No template file found
  exit 1
}
#echo Using $templateFile.tmpl

gpkg=ddl-erd
easygen $templateFile ${gpkg}_cli | tee /tmp/${gpkg}_cliDef.go | goimports > ${gpkg}_cliGen.go
echo ${gpkg}_cliGen.go generated/updated.
