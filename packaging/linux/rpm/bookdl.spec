Name:           bookdl
Version:        1.0.0
Release:        1%{?dist}
Summary:        Multi-source book downloader

License:        MIT
URL:            https://github.com/billmal071/bookdl
Source0:        bookdl-linux-%{arch}

BuildArch:      x86_64

%description
bookdl is a command-line tool for searching and downloading
books from multiple sources: Anna's Archive, Z-Library, and Liber3.

Features:
- Multi-source search
- Resumable downloads
- Cloudflare bypass
- Interactive TUI
- Download management

%prep
# Nothing to prepare for binary package

%install
rm -rf %{buildroot}
mkdir -p %{buildroot}/usr/bin
install -m 755 %{SOURCE0} %{buildroot}/usr/bin/bookdl

%files
%{_bindir}/bookdl

%post
echo ""
echo "bookdl has been installed successfully."
echo ""
echo "To verify installation, run:"
echo "  bookdl --version"
echo ""
echo "For usage information, run:"
echo "  bookdl --help"
echo ""

%changelog
* Mon Apr 15 2024 bookdl <bookdl@github.com> - 1.0.0-1
- Initial package
