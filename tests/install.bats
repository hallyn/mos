load helpers

function setup() {
  common_setup
  zot_setup
}

function teardown() {
  zot_teardown
  common_teardown
}

@test "simple mos install from local oci" {
	skip # good_install uses mosb install which is not yet converted
	good_install hostfsonly
	cat $TMPD/install.yaml
	[ -f $TMPD/atomfs-store/busybox-squashfs/index.json ]
	[ -f $TMPD/config/manifest.git/manifest.yaml ]
}

@test "simple mos install with bad signature" {
	sum=$(manifest_shasum busybox-squashfs)
	cat > $TMPD/install.yaml << EOF
version: 1
product: de6c82c5-2e01-4c92-949b-a6545d30fc06
update_type: complete
targets:
  - service_name: hostfs
    imagepath: puzzleos/hostfs
    version: 1.0.0
    digest: $sum
    service_type: hostfs
    nsgroup: ""
    network:
      type: host
    mounts: []
EOF

	skopeo copy --dest-tls-verify=false oci:zothub:busybox-squashfs docker://$ZOT_HOST:$ZOT_PORT/puzzleos/hostfs:1.0.0
	oras push --plain-http --image-spec v1.1-image $ZOT_HOST:$ZOT_PORT/machine/install:1.0.0 "$TMPD/install.yaml":vnd.machine.install
	echo "fooled ya" > "$TMPD/install.yaml.signed"
	oras attach --plain-http --image-spec v1.1-image --artifact-type vnd.machine.pubkeycrt $ZOT_HOST:$ZOT_PORT/machine/install:1.0.0 "$KEYS_DIR/manifest/cert.pem"
	oras attach --plain-http --image-spec v1.1-image --artifact-type vnd.machine.signature $ZOT_HOST:$ZOT_PORT/machine/install:1.0.0 "$TMPD/install.yaml.signed"
	cp "${KEYS_DIR}/manifest-ca/cert.pem" "$TMPD/manifestCA.pem"
	failed=0
	./mosctl install -c $TMPD/config -a $TMPD/atomfs-store $ZOT_HOST:$ZOT_PORT/machine/install:1.0.0 || failed=1
	[ $failed -eq 1 ]
}

@test "simple mos install from local zot" {
	sum=$(manifest_shasum busybox-squashfs)
	cat > $TMPD/install.yaml << EOF
version: 1
product: de6c82c5-2e01-4c92-949b-a6545d30fc06
update_type: complete
targets:
  - service_name: hostfs
    imagepath: puzzleos/hostfs
    version: 1.0.0
    digest: $sum
    service_type: hostfs
    nsgroup: ""
    network:
      type: host
    mounts: []
EOF

	skopeo copy --dest-tls-verify=false oci:zothub:busybox-squashfs docker://$ZOT_HOST:$ZOT_PORT/puzzleos/hostfs:1.0.0
	oras push --plain-http --image-spec v1.1-image $ZOT_HOST:$ZOT_PORT/machine/install:1.0.0 "$TMPD/install.yaml":vnd.machine.install
	openssl dgst -sha256 -sign "${KEYS_DIR}/manifest/privkey.pem" \
		-out "$TMPD/install.yaml.signed" "$TMPD/install.yaml"
	oras attach --plain-http --image-spec v1.1-image --artifact-type vnd.machine.pubkeycrt $ZOT_HOST:$ZOT_PORT/machine/install:1.0.0 "$KEYS_DIR/manifest/cert.pem"
	oras attach --plain-http --image-spec v1.1-image --artifact-type vnd.machine.signature $ZOT_HOST:$ZOT_PORT/machine/install:1.0.0 "$TMPD/install.yaml.signed"
	cp "${KEYS_DIR}/manifest-ca/cert.pem" "$TMPD/manifestCA.pem"
	./mosctl install --ca-path "$TMPD/manifestCA.pem" -c $TMPD/config -a $TMPD/atomfs-store $ZOT_HOST:$ZOT_PORT/machine/install:1.0.0
	[ -f $TMPD/atomfs-store/puzzleos/hostfs/index.json ]
}

@test "mos install with bad version" {
	sum=$(manifest_shasum busybox-squashfs)
	cat > $TMPD/install.yaml << EOF
version: 2
product: de6c82c5-2e01-4c92-949b-a6545d30fc06
update_type: complete
targets:
EOF
	skopeo copy --dest-tls-verify=false oci:zothub:busybox-squashfs docker://$ZOT_HOST:$ZOT_PORT/puzzleos/hostfs:1.0.0
	oras push --plain-http --image-spec v1.1-image $ZOT_HOST:$ZOT_PORT/machine/install:1.0.0 "$TMPD/install.yaml":vnd.machine.install
	openssl dgst -sha256 -sign "${KEYS_DIR}/manifest/privkey.pem" \
		-out "$TMPD/install.yaml.signed" "$TMPD/install.yaml"
	oras attach --plain-http --image-spec v1.1-image --artifact-type vnd.machine.pubkeycrt $ZOT_HOST:$ZOT_PORT/machine/install:1.0.0 "$KEYS_DIR/manifest/cert.pem"
	oras attach --plain-http --image-spec v1.1-image --artifact-type vnd.machine.signature $ZOT_HOST:$ZOT_PORT/machine/install:1.0.0 "$TMPD/install.yaml.signed"
	cp "${KEYS_DIR}/manifest-ca/cert.pem" "$TMPD/manifestCA.pem"

	failed=0
	./mosctl install -c $TMPD/config -a $TMPD/atomfs-store $ZOT_HOST:$ZOT_PORT/machine/install:1.0.0 || failed=1
	[ $failed -eq 1 ]
}

@test "simple mos install with bad manifest hash" {
	sum=$(manifest_shasum busybox-squashfs)
	# Next line is where we make the manifest hash invalid
	sum=$(echo $sum | sha256sum | cut -f 1 -d \ )
	cat > $TMPD/install.yaml << EOF
version: 1
product: de6c82c5-2e01-4c92-949b-a6545d30fc06
update_type: complete
targets:
  - service_name: hostfs
    imagepath: puzzleos/hostfs
    version: 1.0.0
    digest: $sum
    service_type: hostfs
    nsgroup: ""
    network:
      type: host
    mounts: []
EOF
	skopeo copy --dest-tls-verify=false oci:zothub:busybox-squashfs docker://$ZOT_HOST:$ZOT_PORT/puzzleos/hostfs:1.0.0
	oras push --plain-http --image-spec v1.1-image $ZOT_HOST:$ZOT_PORT/machine/install:1.0.0 "$TMPD/install.yaml":vnd.machine.install
	openssl dgst -sha256 -sign "${KEYS_DIR}/manifest/privkey.pem" \
		-out "$TMPD/install.yaml.signed" "$TMPD/install.yaml"
	oras attach --plain-http --image-spec v1.1-image --artifact-type vnd.machine.pubkeycrt $ZOT_HOST:$ZOT_PORT/machine/install:1.0.0 "$KEYS_DIR/manifest/cert.pem"
	oras attach --plain-http --image-spec v1.1-image --artifact-type vnd.machine.signature $ZOT_HOST:$ZOT_PORT/machine/install:1.0.0 "$TMPD/install.yaml.signed"
	cp "${KEYS_DIR}/manifest-ca/cert.pem" "$TMPD/manifestCA.pem"

	failed=0
	./mosctl install -c $TMPD/config -a $TMPD/atomfs-store $ZOT_HOST:$ZOT_PORT/machine/install:1.0.0 || failed=1
	[ $failed -eq 1 ]
}
