#!/bin/bash
echo "--- PRE-FLUSH MEMORY STATUS ---"
free -h

echo -e "\n1. Flushing file system buffers to disk (sync)..."
sudo sync

echo "2. Dropping PageCache, dentries, and inodes..."
# 1 = PageCache, 2 = dentries/inodes, 3 = All
echo 3 | sudo tee /proc/sys/vm/drop_caches > /dev/null

echo -e "3. (Optional) Refreshing Swap..."
# Only do this if you actually used swap; moves data back to RAM
sudo swapoff -a && sudo swapon -a

echo -e "\n--- POST-FLUSH MEMORY STATUS ---"
free -h
echo -e "\nBunker memory is now pristine. Ready for the next model load."