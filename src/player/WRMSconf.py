import logging
import os

music_dir = "/tmp/WRMS"

if not os.path.isdir(music_dir):
    os.mkdir(music_dir)

logfile = music_dir + "/default.log"
loglevel = logging.INFO
