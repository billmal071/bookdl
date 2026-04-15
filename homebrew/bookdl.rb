class Bookdl < Formula
  desc "Multi-source book downloader for Anna's Archive, Z-Library, and Liber3"
  homepage "https://github.com/billmal071/bookdl"
  version "0.1.0" # Update this when releasing
  license "MIT"

  on_macos do
    on_intel do
      url "https://github.com/billmal071/bookdl/releases/download/v#{version}/bookdl-darwin-amd64"
      sha256 "" # Add SHA256 after release
    end
    on_arm do
      url "https://github.com/billmal071/bookdl/releases/download/v#{version}/bookdl-darwin-arm64"
      sha256 "" # Add SHA256 after release
    end
  end

  on_linux do
    on_intel do
      url "https://github.com/billmal071/bookdl/releases/download/v#{version}/bookdl-linux-amd64"
      sha256 "" # Add SHA256 after release
    end
    on_arm do
      url "https://github.com/billmal071/bookdl/releases/download/v#{version}/bookdl-linux-arm64"
      sha256 "" # Add SHA256 after release
    end
  end

  def install
    bin.install "bookdl-darwin-amd64" => "bookdl" if Hardware::CPU.intel?
    bin.install "bookdl-darwin-arm64" => "bookdl" if Hardware::CPU.arm? && OS.mac?
    bin.install "bookdl-linux-amd64" => "bookdl" if Hardware::CPU.intel? && OS.linux?
    bin.install "bookdl-linux-arm64" => "bookdl" if Hardware::CPU.arm? && OS.linux?
  end

  test do
    assert_match "bookdl version", shell_output("#{bin}/bookdl --version")
  end
end
