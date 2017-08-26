#!/usr/bin/perl

use strict;
use warnings;
use File::Copy qw(copy);

local $ENV{PATH} = "$ENV{PATH};C:/Program Files/7-Zip";

my $version = '1.2';
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
        my $bin = "${binary}_v${version}_${o}_${a}";
        `go build -o tmp/${binary}${ext}`;

        if ($o eq 'windows') {
            `7z a builds/${bin}.zip ./tmp/*`;
        } else {
            `7z a builds/${bin}.tar ./tmp/*`;
            `7z a builds/${bin}.tar.gz ./builds/${bin}.tar`;
            unlink "./builds/${bin}.tar";
        }

        unlink "./tmp/${bin}${ext}";
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
