TAR_PREFIX=gnmiClient
PLATFORM=$(uname)
ARCH=$(uname -m)
LATEST_URL=$(curl -s https://github.com/karimra/gnmiClient/releases/latest | cut -d '"' -f 2)
LATEST_TAG=$(echo "${LATEST_URL: -6}")
TAG_WO_VER=$(echo "${LATEST_URL: -6}" | cut -c 2-)
FNAME="${TAR_PREFIX}_${TAG_WO_VER}_${PLATFORM}_${ARCH}.tar.gz"
echo "Downloading $FNAME..."
(cd /tmp && curl -ksLO https://github.com/karimra/gnmiClient/releases/download/"$LATEST_TAG"/"$FNAME")
tar -xzf /tmp/${FNAME} -C /tmp
echo "Moving gnmiClient to /usr/local/bin"
echo
mv /tmp/gnmiClient /usr/local/bin
/usr/local/bin/gnmiClient version
echo
echo "Installation complete!"
