# Go ALSA bindings

These bindings allow capture and playback of audio via
[ALSA](http://www.alsa-project.org/) using the
[alsa-lib](http://www.alsa-project.org/alsa-doc/alsa-lib/pcm.html) library.

[![Build Status](https://travis-ci.org/cocoonlife/goalsa.svg)](https://travis-ci.org/cocoonlife/goalsa)

[![Coverage Status](https://coveralls.io/repos/cocoonlife/goalsa/badge.svg?branch=master&service=github)](https://coveralls.io/github/cocoonlife/goalsa?branch=master)

### Installation

    go get github.com/cocoonlife/goalsa

### Status

The code has support for capture and playback with various parameters
however it is only quite lightly tested so it is likely that bugs remain.
Playback in particular has not been very well tested.
