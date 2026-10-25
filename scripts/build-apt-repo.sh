#!/usr/bin/env bash
# Builds (or updates) a signed apt repository in a directory, from the .deb files attached
# to GitHub releases. Used by .github/workflows/apt-repo.yml; safe to run locally.
#
# Usage: scripts/build-apt-repo.sh SITE_DIR GPG_KEY_ID TAG [TAG...]
#
# SITE_DIR   directory that becomes the web root (the gh-pages branch checkout). Debs from
#            earlier runs already in it are kept, so older versions stay installable.
# GPG_KEY_ID signing key, already imported into the current GnuPG home (no passphrase).
# TAG        release tag(s) to download .deb files from, e.g. v0.1.3. Needs `gh` logged in
#            and GH_REPO (owner/name) set.
#
# Layout produced (suite "stable", component "main", architectures amd64 and arm64):
#   dists/stable/{Release,InRelease,Release.gpg}
#   dists/stable/main/binary-<arch>/{Packages,Packages.gz}
#   pool/main/<arch>/*.deb
#   key.gpg (binary, for /usr/share/keyrings) and key.asc (armored)
set -euo pipefail

site=${1:?site dir}
key=${2:?gpg key id}
shift 2
[ $# -ge 1 ] || { echo "need at least one release tag" >&2; exit 2; }

archs=(amd64 arm64)
mkdir -p "$site"
cd "$site"

for tag in "$@"; do
  tmp=$(mktemp -d)
  gh release download "$tag" --pattern '*.deb' --dir "$tmp"
  for arch in "${archs[@]}"; do
    mkdir -p "pool/main/$arch"
    # Release assets are named term-achievements_<version>_linux_<arch>.deb.
    copied=0
    for deb in "$tmp"/*_linux_"$arch".deb; do
      if [ -e "$deb" ]; then
        cp "$deb" "pool/main/$arch/"
        copied=$((copied + 1))
      fi
    done
    if [ "$copied" -eq 0 ]; then
      echo "error: no .deb packages found in release $tag for arch $arch" >&2
      rm -r "$tmp"
      exit 1
    fi
  done
  rm -r "$tmp"
done

for arch in "${archs[@]}"; do
  dir="dists/stable/main/binary-$arch"
  mkdir -p "$dir"
  apt-ftparchive packages "pool/main/$arch" > "$dir/Packages"
  gzip -9nkf "$dir/Packages"
done

apt-ftparchive \
  -o APT::FTPArchive::Release::Origin=term-achievements \
  -o APT::FTPArchive::Release::Label=term-achievements \
  -o APT::FTPArchive::Release::Suite=stable \
  -o APT::FTPArchive::Release::Codename=stable \
  -o APT::FTPArchive::Release::Architectures="${archs[*]}" \
  -o APT::FTPArchive::Release::Components=main \
  release dists/stable > dists/stable/Release.new
mv dists/stable/Release.new dists/stable/Release

gpg --batch --yes --pinentry-mode loopback --passphrase '' -u "$key" \
  --clearsign -o dists/stable/InRelease dists/stable/Release
gpg --batch --yes --pinentry-mode loopback --passphrase '' -u "$key" \
  --armor --detach-sign -o dists/stable/Release.gpg dists/stable/Release

gpg --export "$key" > key.gpg
gpg --armor --export "$key" > key.asc

cat > index.html <<'HTML'
<!doctype html>
<meta charset="utf-8">
<title>term-achievements apt repository</title>
<h1>term-achievements apt repository</h1>
<p>Xbox-style achievements for your shell. Source: <a href="https://github.com/dpologdvinity/term-achievements">github.com/dpologdvinity/term-achievements</a></p>
<pre>
curl -fsSL https://dpologdvinity.github.io/term-achievements/key.gpg | sudo tee /usr/share/keyrings/term-achievements.gpg &gt;/dev/null
echo "deb [signed-by=/usr/share/keyrings/term-achievements.gpg] https://dpologdvinity.github.io/term-achievements stable main" | sudo tee /etc/apt/sources.list.d/term-achievements.list
sudo apt update
sudo apt install term-achievements
</pre>
HTML

# GitHub Pages must serve the files as they are (no Jekyll processing).
touch .nojekyll
