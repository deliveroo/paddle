version=`cat ../VERSION`
# Write out the package.
cat << EOF > version.go
package cli
//go:generate bash ./generate_version.sh
var PaddleVersion = "$version"
EOF
