load helpers

function setup() {
	common_setup
	zot_setup
}

function teardown() {
	zot_teardown
	common_teardown
}

@test "simple mos update from local zot" {
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

	mkdir -p $TMPD/factory/secure
	cp ${KEYS_DIR}/manifest-ca/cert.pem $TMPD/factory/secure/manifestCA.pem
	./mosctl install --ca-path "$TMPD/factory/secure/manifestCA.pem" -c $TMPD/config -a $TMPD/atomfs-store $ZOT_HOST:$ZOT_PORT/machine/install:1.0.0
	[ -f $TMPD/atomfs-store/puzzleos/hostfs/index.json ]
	sum=$(manifest_shasum busyboxu1-squashfs)
	cat > $TMPD/install.yaml << EOF
version: 1
product: de6c82c5-2e01-4c92-949b-a6545d30fc06
update_type: complete
targets:
  - service_name: hostfs
    imagepath: puzzleos/hostfs
    version: 1.0.2
    digest: $sum
    service_type: hostfs
    nsgroup: ""
    network:
      type: host
    mounts: []
EOF
	oras push --plain-http --image-spec v1.1-image $ZOT_HOST:$ZOT_PORT/machine/install:1.0.2 "$TMPD/install.yaml":vnd.machine.install
	skopeo copy --dest-tls-verify=false oci:zothub:busyboxu1-squashfs docker://$ZOT_HOST:$ZOT_PORT/puzzleos/hostfs:1.0.2
	openssl dgst -sha256 -sign "${KEYS_DIR}/manifest/privkey.pem" \
		-out "$TMPD/install.yaml.signed" "$TMPD/install.yaml"
	oras attach --plain-http --image-spec v1.1-image --artifact-type vnd.machine.pubkeycrt $ZOT_HOST:$ZOT_PORT/machine/install:1.0.2 "$KEYS_DIR/manifest/cert.pem"
	oras attach --plain-http --image-spec v1.1-image --artifact-type vnd.machine.signature $ZOT_HOST:$ZOT_PORT/machine/install:1.0.2 "$TMPD/install.yaml.signed"
	./mosctl update -r $TMPD $ZOT_HOST:$ZOT_PORT/machine/install:1.0.2
}

@test "update of fs-only layer" {
	# Simple install
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
  - service_name: hostfstarget
    imagepath: puzzleos/hostfstarget
    version: 1.0.0
    digest: $sum
    service_type: fs-only
    nsgroup: ""
    network:
      type: none
    mounts: []
EOF
	oras push --plain-http --image-spec v1.1-image $ZOT_HOST:$ZOT_PORT/machine/install:1.0.0 "$TMPD/install.yaml":vnd.machine.install
	openssl dgst -sha256 -sign "${KEYS_DIR}/manifest/privkey.pem" \
		-out "$TMPD/install.yaml.signed" "$TMPD/install.yaml"
	oras attach --plain-http --image-spec v1.1-image --artifact-type vnd.machine.pubkeycrt $ZOT_HOST:$ZOT_PORT/machine/install:1.0.0 "$KEYS_DIR/manifest/cert.pem"
	oras attach --plain-http --image-spec v1.1-image --artifact-type vnd.machine.signature $ZOT_HOST:$ZOT_PORT/machine/install:1.0.0 "$TMPD/install.yaml.signed"
	skopeo copy --dest-tls-verify=false oci:zothub:busybox-squashfs docker://$ZOT_HOST:$ZOT_PORT/puzzleos/hostfs:1.0.0
	skopeo copy --dest-tls-verify=false oci:zothub:busybox-squashfs docker://$ZOT_HOST:$ZOT_PORT/puzzleos/hostfstarget:1.0.0
	# In "real life", /factory/secure/ is set up by the signed initrd
	mkdir -p $TMPD/factory/secure
	cp ${KEYS_DIR}/manifest-ca/cert.pem $TMPD/factory/secure/manifestCA.pem
	./mosctl install --ca-path "$TMPD/factory/secure/manifestCA.pem" -c $TMPD/config -a $TMPD/atomfs-store $ZOT_HOST:$ZOT_PORT/machine/install:1.0.0
	export TMPD
	lxc-usernsexec -s -- << "EOF"
unshare -m -- << "XXX"
#!/bin/bash
set -e
./mosctl activate -r $TMPD -t hostfstarget -capath $TMPD/factory/secure/manifestCA.pem
[ -e $TMPD/mnt/atom/hostfstarget/etc ]
/bin/ls -l $TMPD/mnt/atom/hostfstarget
cat /proc/self/mountinfo
killall squashfuse || true
XXX
EOF

	# Now upgrade
	sum=$(manifest_shasum busyboxu1-squashfs)
	cat > $TMPD/install.yaml << EOF
version: 1
product: de6c82c5-2e01-4c92-949b-a6545d30fc06
update_type: complete
targets:
  - service_name: hostfs
    imagepath: puzzleos/hostfs
    version: 1.0.2
    digest: $sum
    service_type: hostfs
    nsgroup: ""
    network:
      type: host
    mounts: []
  - service_name: hostfstarget
    imagepath: puzzleos/hostfstarget
    version: 1.0.2
    digest: $sum
    service_type: fs-only
    nsgroup: ""
    network:
      type: none
    mounts: []
EOF
	oras push --plain-http --image-spec v1.1-image $ZOT_HOST:$ZOT_PORT/machine/install:1.0.2 "$TMPD/install.yaml":vnd.machine.install
	skopeo copy --dest-tls-verify=false oci:zothub:busyboxu1-squashfs docker://$ZOT_HOST:$ZOT_PORT/puzzleos/hostfs:1.0.2
	skopeo copy --dest-tls-verify=false oci:zothub:busyboxu1-squashfs docker://$ZOT_HOST:$ZOT_PORT/puzzleos/hostfstarget:1.0.2
	openssl dgst -sha256 -sign "${KEYS_DIR}/manifest/privkey.pem" \
		-out "$TMPD/install.yaml.signed" "$TMPD/install.yaml"
	oras attach --plain-http --image-spec v1.1-image --artifact-type vnd.machine.pubkeycrt $ZOT_HOST:$ZOT_PORT/machine/install:1.0.2 "$KEYS_DIR/manifest/cert.pem"
	oras attach --plain-http --image-spec v1.1-image --artifact-type vnd.machine.signature $ZOT_HOST:$ZOT_PORT/machine/install:1.0.2 "$TMPD/install.yaml.signed"
	echo "BEFORE UPDATE"
	ls -l $TMPD/config/manifest.git
	(cd $TMPD/config/manifest.git; git status)
	echo "END OF BEFORE UPDATE"
	./mosctl update -r $TMPD $ZOT_HOST:$ZOT_PORT/machine/install:1.0.2

	ls -l $TMPD/config/manifest.git
	(cd $TMPD/config/manifest.git; git status)
	# And test, making sure the 'u1' file is there
	lxc-usernsexec -s -- << "EOF"
unshare -m -- << "XXX"
#!/bin/bash
set -e
./mosctl activate -r $TMPD -t hostfstarget -capath $TMPD/factory/secure/manifestCA.pem
[ -e $TMPD/mnt/atom/hostfstarget/etc ]
/bin/ls -l $TMPD/mnt/atom/hostfstarget
cat /proc/self/mountinfo
[ -e $TMPD/mnt/atom/hostfstarget/u1 ]
killall squashfuse || true
XXX
EOF
}

@test "test partial update" {
	# Simple install
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
	oras push --plain-http --image-spec v1.1-image $ZOT_HOST:$ZOT_PORT/machine/install:1.0.0 "$TMPD/install.yaml":vnd.machine.install
	openssl dgst -sha256 -sign "${KEYS_DIR}/manifest/privkey.pem" \
		-out "$TMPD/install.yaml.signed" "$TMPD/install.yaml"
	oras attach --plain-http --image-spec v1.1-image --artifact-type vnd.machine.pubkeycrt $ZOT_HOST:$ZOT_PORT/machine/install:1.0.0 "$KEYS_DIR/manifest/cert.pem"
	oras attach --plain-http --image-spec v1.1-image --artifact-type vnd.machine.signature $ZOT_HOST:$ZOT_PORT/machine/install:1.0.0 "$TMPD/install.yaml.signed"
	skopeo copy --dest-tls-verify=false oci:zothub:busybox-squashfs docker://$ZOT_HOST:$ZOT_PORT/puzzleos/hostfs:1.0.0
	mkdir -p $TMPD/factory/secure
	cp ${KEYS_DIR}/manifest-ca/cert.pem $TMPD/factory/secure/manifestCA.pem
	./mosctl install --ca-path "$TMPD/factory/secure/manifestCA.pem" -c $TMPD/config -a $TMPD/atomfs-store $ZOT_HOST:$ZOT_PORT/machine/install:1.0.0

	# Now do a partial upgrade to install hostfstarget
	sum=$(manifest_shasum busyboxu1-squashfs)
	cat > $TMPD/install.yaml << EOF
version: 1
product: de6c82c5-2e01-4c92-949b-a6545d30fc06
update_type: partial
targets:
  - service_name: hostfstarget
    imagepath: puzzleos/hostfstarget
    version: 1.0.2
    digest: $sum
    service_type: fs-only
    nsgroup: ""
    network:
      type: none
    mounts: []
EOF
	skopeo copy --dest-tls-verify=false oci:zothub:busyboxu1-squashfs docker://$ZOT_HOST:$ZOT_PORT/puzzleos/hostfstarget:1.0.2
	oras push --plain-http --image-spec v1.1-image $ZOT_HOST:$ZOT_PORT/machine/install:1.0.2 "$TMPD/install.yaml":vnd.machine.install
	openssl dgst -sha256 -sign "${KEYS_DIR}/manifest/privkey.pem" \
		-out "$TMPD/install.yaml.signed" "$TMPD/install.yaml"
	oras attach --plain-http --image-spec v1.1-image --artifact-type vnd.machine.pubkeycrt $ZOT_HOST:$ZOT_PORT/machine/install:1.0.2 "$KEYS_DIR/manifest/cert.pem"
	oras attach --plain-http --image-spec v1.1-image --artifact-type vnd.machine.signature $ZOT_HOST:$ZOT_PORT/machine/install:1.0.2 "$TMPD/install.yaml.signed"
	echo "BEFORE UPDATE"
	ls -l $TMPD/config/manifest.git
	(cd $TMPD/config/manifest.git; git status)
	echo "END OF BEFORE UPDATE"
	./mosctl update -r $TMPD $ZOT_HOST:$ZOT_PORT/machine/install:1.0.2
	echo "AFTER UPDATE"
	ls -l $TMPD/config/manifest.git
	(cd $TMPD/config/manifest.git; git status; git log)
	echo "AFTER OF BEFORE UPDATE"

	# Test, make sure the 'u1' file is there in hostfstarget
	lxc-usernsexec -s -- << "EOF"
unshare -m -- << "XXX"
#!/bin/bash
set -e
./mosctl activate -r $TMPD -t hostfstarget -capath $TMPD/factory/secure/manifestCA.pem
[ -e $TMPD/mnt/atom/hostfstarget/etc ]
/bin/ls -l $TMPD/mnt/atom/hostfstarget
cat /proc/self/mountinfo
# Re-activate, to test stop
./mosctl activate -r $TMPD -t hostfstarget -capath $TMPD/factory/secure/manifestCA.pem
[ -e $TMPD/mnt/atom/hostfstarget/etc ]
[ -e $TMPD/mnt/atom/hostfstarget/u1 ]
killall squashfuse || true
XXX
EOF

	# Also make sure we can still mount the hostfs
	mkdir -p "${TMPD}/mnt"
	export TMPD
	lxc-usernsexec -s -- << "EOF"
unshare -m -- << "XXX"
#!/bin/bash
set -e
./mosctl create-boot-fs --readonly -c $TMPD/config -a $TMPD/atomfs-store \
   -s $TMPD/scratch-writes --ca-path $TMPD/factory/secure/manifestCA.pem --dest $TMPD/mnt
sleep 1s
[ -e $TMPD/mnt/etc ]
failed=0
echo testing > $TMPD/mnt/helloworld || failed=1
[ $failed -eq 1 ]
killall squashfuse || true
XXX
EOF
}
