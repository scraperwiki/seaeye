#!/bin/sh

# Migrate volume mounted SSH keys
cp /seaeye/ssh/* /home/nobody/.ssh

# Start service
seaeye $@
