%global debug_package %{nil}
%define __strip /bin/true

Name:           go-flow
Version:        %{_flow_version}
Release:        %{_flow_release}%{?dist}
Summary:        ETL Madness Flow pipeline engine, crypto_tool, and db_importer
License:        MIT
URL:            https://github.com/etl-madness/go-flow
Source0:        go-flow-%{_flow_version}-staged.tar.gz

%description
ETL Madness Flow is an XML-driven ETL and data workflow ecosystem installed
under /opt/flow so its utilities never conflict with existing system packages:
- /opt/flow/bin/flow: Core pipeline execution engine, validator, HTMX visual
  builder server, XSLT/Mermaid diagram generator, and repository manager.
- /opt/flow/bin/crypto_tool: Standalone AES-256-GCM encryption/decryption,
  digital signing (OpenSSL, OpenPGP, PKCS#7/CMS), verification, and keygen tool.
- /opt/flow/bin/db_importer: SQL Server XML importer and exporter.

%prep
%setup -q -c -n go-flow-%{_flow_version}

%build
# Pre-compiled pure-Go static binaries and compressed docs are staged in Source0.

%install
rm -rf %{buildroot}
install -d -m 0755 %{buildroot}/opt/flow/bin
install -m 0755 bin/flow %{buildroot}/opt/flow/bin/flow
install -m 0755 bin/crypto_tool %{buildroot}/opt/flow/bin/crypto_tool
install -m 0755 bin/db_importer %{buildroot}/opt/flow/bin/db_importer

install -d -m 0755 %{buildroot}/opt/flow/share/man/man1
install -m 0644 doc/man1/flow.1.gz %{buildroot}/opt/flow/share/man/man1/flow.1.gz
install -m 0644 doc/man1/crypto_tool.1.gz %{buildroot}/opt/flow/share/man/man1/crypto_tool.1.gz
install -m 0644 doc/man1/db_importer.1.gz %{buildroot}/opt/flow/share/man/man1/db_importer.1.gz

install -d -m 0755 %{buildroot}/opt/flow/share/man/man5
install -m 0644 doc/man5/flow-pipeline.5.gz %{buildroot}/opt/flow/share/man/man5/flow-pipeline.5.gz

install -d -m 0755 %{buildroot}/opt/flow/share/info
install -m 0644 doc/info/flow.info.gz %{buildroot}/opt/flow/share/info/flow.info.gz
install -m 0644 doc/info/dir %{buildroot}/opt/flow/share/info/dir

install -d -m 0755 %{buildroot}/opt/flow/share/xsd
install -m 0644 xsd/pipeline.xsd %{buildroot}/opt/flow/share/xsd/pipeline.xsd
install -m 0644 xsd/options.xsd %{buildroot}/opt/flow/share/xsd/options.xsd

install -d -m 0755 %{buildroot}/opt/flow/share/licenses
install -m 0644 LICENSE %{buildroot}/opt/flow/share/licenses/LICENSE

install -d -m 0755 %{buildroot}/etc/profile.d
install -m 0644 profile.d/go-flow.sh %{buildroot}/etc/profile.d/go-flow.sh

%files
%license /opt/flow/share/licenses/LICENSE
%dir /opt/flow
%dir /opt/flow/bin
/opt/flow/bin/flow
/opt/flow/bin/crypto_tool
/opt/flow/bin/db_importer
%dir /opt/flow/share
%dir /opt/flow/share/man
%dir /opt/flow/share/man/man1
%dir /opt/flow/share/man/man5
/opt/flow/share/man/man1/flow.1*
/opt/flow/share/man/man1/crypto_tool.1*
/opt/flow/share/man/man1/db_importer.1*
/opt/flow/share/man/man5/flow-pipeline.5*
%dir /opt/flow/share/info
/opt/flow/share/info/flow.info*
/opt/flow/share/info/dir
%dir /opt/flow/share/xsd
/opt/flow/share/xsd/pipeline.xsd
/opt/flow/share/xsd/options.xsd
%dir /opt/flow/share/licenses
%config(noreplace) /etc/profile.d/go-flow.sh
