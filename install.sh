set -e
TAR_PREFIX=gnmic
PLATFORM=$(uname)
ARCH=$(uname -m)
INSTALLED_VERSION=$(gnmic version 2>/dev/null | grep version | awk '{print $3}') || ""
LATEST_URL=$(curl -s https://github.com/karimra/gnmic/releases/latest | cut -d '"' -f 2)
LATEST_TAG=$(echo "${LATEST_URL: -6}")
TAG_WO_VER=$(echo "${LATEST_URL: -6}" | cut -c 2-)
if [ "$INSTALLED_VERSION" == "$TAG_WO_VER" ]; then
    echo "You have the latest gnmic version installed: $INSTALLED_VERSION"
    exit 0
fi
FNAME="${TAR_PREFIX}_${TAG_WO_VER}_${PLATFORM}_${ARCH}.tar.gz"
echo "Downloading $FNAME..."
(cd /tmp && curl -ksLO https://github.com/karimra/gnmic/releases/download/"$LATEST_TAG"/"$FNAME" || (echo "Failed to download release!" && exit 1))
tar -xzf /tmp/${FNAME} -C /tmp || (echo "Failed to unarchive!" && exit 1)
echo "Moving gnmic to /usr/local/bin"
echo
mv -f /tmp/gnmic /usr/local/bin
/usr/local/bin/gnmic version
echo
echo "Installation complete!"
