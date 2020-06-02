TAR_PREFIX=gnmiClient
PLATFORM=$(uname)
ARCH=$(uname -m)
INSTALLED_VERSION=$(gnmiClient version 2>/dev/null | grep version | awk '{print $3}') || ""
LATEST_URL=$(curl -s https://github.com/karimra/gnmiClient/releases/latest | cut -d '"' -f 2)
LATEST_TAG=$(echo "${LATEST_URL: -6}")
TAG_WO_VER=$(echo "${LATEST_URL: -6}" | cut -c 2-)
if [ "$INSTALLED_VERSION" == "$TAG_WO_VER" ]; then
    echo "You have the latest gnmiClient version installed: $INSTALLED_VERSION"
    exit 0
fi
FNAME="${TAR_PREFIX}_${TAG_WO_VER}_${PLATFORM}_${ARCH}.tar.gz"
echo "Downloading $FNAME..."
(cd /tmp && curl -ksLO https://github.com/karimra/gnmiClient/releases/download/"$LATEST_TAG"/"$FNAME")
tar -xzf /tmp/${FNAME} -C /tmp
echo "Moving gnmiClient to /usr/local/bin"
echo
mv -f /tmp/gnmiClient /usr/local/bin
/usr/local/bin/gnmiClient version
echo
echo "Installation complete!"
