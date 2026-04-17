# Homebrew formula for kennel (Go rewrite).
#
# Publish path: kentny/homebrew-tap/Formula/kennel.rb
# Users install with: brew install kentny/tap/kennel
#
# For the v0.1.0 manual release:
#   1. Tag and cut the GitHub Release (goreleaser or by hand).
#   2. Copy the four architecture-specific `.tar.gz` URLs here, along with
#      the sha256 from `checksums.txt`.
#   3. Commit this file to kentny/homebrew-tap.
#
# Once goreleaser's `brews:` block is enabled (.goreleaser.yaml), step 2-3
# happen automatically per tag and this file stops being edited by hand.
class Kennel < Formula
  desc "Portable sandbox manager for AI coding agents (Claude, Codex)"
  homepage "https://github.com/kentny/kennel"
  version "0.1.0"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/kentny/kennel/releases/download/v0.1.0/kennel_0.1.0_darwin_arm64.tar.gz"
      sha256 "REPLACE_WITH_RELEASE_SHA256"
    end
    on_intel do
      url "https://github.com/kentny/kennel/releases/download/v0.1.0/kennel_0.1.0_darwin_amd64.tar.gz"
      sha256 "REPLACE_WITH_RELEASE_SHA256"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/kentny/kennel/releases/download/v0.1.0/kennel_0.1.0_linux_arm64.tar.gz"
      sha256 "REPLACE_WITH_RELEASE_SHA256"
    end
    on_intel do
      url "https://github.com/kentny/kennel/releases/download/v0.1.0/kennel_0.1.0_linux_amd64.tar.gz"
      sha256 "REPLACE_WITH_RELEASE_SHA256"
    end
  end

  def install
    bin.install "kennel"
  end

  def caveats
    <<~EOS
      kennel shells out to the `docker sandbox` command group, which requires
      Docker Desktop 4.58 or newer. Install it from:
        https://www.docker.com/products/docker-desktop/

      Get started:
        cd your-project
        kennel init
        kennel build
        kennel run
    EOS
  end

  test do
    assert_match "kennel", shell_output("#{bin}/kennel version")
    assert_match "Usage", shell_output("#{bin}/kennel --help")
  end
end
