
%define version_subrelease 1
%define buildtime       %(echo `LC_ALL="C" date +"%Y%m%d%H%M"`)
%define myrelease       %{version_subrelease}.%{buildtime}%{?dist}

Name:		gnmic
Version:	0.31.6
Release:	%{myrelease}
Summary:	gNMIc is a gNMI CLI client and collector

Group:		Applications/System
License:	ASL 2.0
URL:		https://gnmic.openconfig.net/
Source0:	gnmic.tar

BuildRoot: %{_tmppath}/%{name}-buildroot-%{whoami}-%{buildhost}/

#BuildRequires:	
#Requires:	

%description
gnmic (pronoun.: gee·en·em·eye·see) is a gNMI CLI client that provides
full support for Capabilities, Get, Set and Subscribe RPCs with
collector capabilities.

%prep
%setup -q -c -n %{name}
rm -rf $RPM_BUILD_ROOT



%install
install -D %{name} $RPM_BUILD_ROOT%{_bindir}/%{name}



%files
%{_bindir}/%{name}



%changelog
* Wed Aug 02 2023 Kaj Niemi <kajtzu@basen.net> 0.9.1-1.0.el9
- first build


