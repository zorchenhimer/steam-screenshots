#!/usr/bin/perl

use strict;
use warnings;
use File::Copy qw(copy);

local $ENV{PATH} = "$ENV{PATH};C:/Program Files/7-Zip";

my $githash = `git rev-parse HEAD`;
chomp $githash;

if (system("git diff --quiet HEAD --") != 0 ) {
    $githash = 'DIRTY@'. $githash;
}

my $version = `git describe --abbrev=0 --tags`;
chomp $version;
my $binary = 'steam-screenshots';
my @OS = (
    'windows',
    'linux',
    'darwin',
    'freebsd',
);

my @ARCH = (
    '386',
    'amd64',
    'arm',
);

my @static = (
    'README.md',
    'LICENSE.txt',
    'settings_example.json',
    'static/',
    'static/default-skin/',
    'templates/',
    'banners/unknown.jpg',
);

mkdir 'tmp';
mkdir 'tmp/banners';
mkdir 'builds';

foreach my $s (@static) {
    if (-d $s) {
        mkdir "tmp/$s";
        my @lst = glob("$s/*");
        foreach (@lst) {
            copy($_, "tmp/$_");
        }
    } else {
        copy($s, "tmp/$s");
    }
}

foreach my $o (@OS) {
    my $ext = '';
    $ext = '.exe' if ($o eq 'windows');

    $ENV{'GOOS'} = $o;

    foreach my $a (@ARCH) {
        next if ($a eq 'arm' && $o ne 'linux');

        $ENV{'GOARCH'} = $a;

        print "Building ${o}/${a}\n";
        my $bin = "${binary}_${version}_${o}_${a}";
        `go build -ldflags "-X main.gitCommit=${githash} -X main.version=${version}" -o tmp/${binary}${ext}`;

        if ($o eq 'windows') {
            `7z a builds/${bin}.zip ./tmp/*`;
        } else {
            `7z a builds/${bin}.tar ./tmp/*`;
            `7z a builds/${bin}.tar.gz ./builds/${bin}.tar`;
            unlink "./builds/${bin}.tar";
        }

        unlink "./tmp/${binary}${ext}";
    }
}

foreach my $s (@static) {
    if (-d $s) {
        unlink glob "./tmp/$s/*";
        rmdir "./tmp/$s";
    } else {
        unlink "./tmp/$s";
    }
}

rmdir './tmp/banners/';
unlink glob "./tmp/*";
rmdir "./tmp";
