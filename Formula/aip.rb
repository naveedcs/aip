class Aip < Formula
  desc "AI profile manager for isolating AI CLI accounts, credentials, and MCP servers"
  homepage "https://github.com/naveedcs/aip"
  version "0.1.1"
  license "MIT"

  on_macos do
    on_arm do
      url "https://github.com/naveedcs/aip/releases/download/v0.1.1/aip_0.1.1_darwin_arm64.tar.gz"
      sha256 "05c7e3c8274cba206508251305c28daa91c211b8125c4a5d1a23550946c0ea98"
    end

    on_intel do
      url "https://github.com/naveedcs/aip/releases/download/v0.1.1/aip_0.1.1_darwin_amd64.tar.gz"
      sha256 "e4f24d0c93dda8d2b2f1ed2dbf3aabe1b739d0de4a34d710b62ce9f8ac8caf0e"
    end
  end

  on_linux do
    on_arm do
      url "https://github.com/naveedcs/aip/releases/download/v0.1.1/aip_0.1.1_linux_arm64.tar.gz"
      sha256 "10ef21991016ef563f7dfa9ac71e19c57d8cf8c4ba8244e249f4a9ba767335d3"
    end

    on_intel do
      url "https://github.com/naveedcs/aip/releases/download/v0.1.1/aip_0.1.1_linux_amd64.tar.gz"
      sha256 "46c43fe3ba07af4e982cffe987c91e10571541026024263f53b0eb712866c4ce"
    end
  end

  def install
    bin.install "aip"
  end

  test do
    assert_match "aip version #{version}", shell_output("#{bin}/aip --version")
  end
end
