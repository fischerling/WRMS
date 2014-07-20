#!/usr/bin/python3
import os

import gi
gi.require_version('Gst', '1.0')
from gi.repository import GObject, Gst

GObject.threads_init()
Gst.init(None)

import dbus
import dbus.service
import dbus.mainloop.glib

dbus.mainloop.glib.DBusGMainLoop(set_as_default=True)
dbus.mainloop.glib.threads_init()

from queue import Dyn_Queue
from song import Song
import WRMSconf

import logging
logging.basicConfig(filename = WRMSconf.logfile,
                    level = WRMSconf.loglevel, filemode = 'w')

music_dir = WRMSconf.music_dir

class Player(dbus.service.Object):
    def __init__(self, playlist = None):
        if playlist:
            self.queue = playlist
        else:
            self.queue = Dyn_Queue()

        self.current_song = None
        self.current_song_taglist = None
        # gst
        # Create our player object
        self.player = Gst.ElementFactory.make('playbin', None)
        
        # connect a signal handler to it's bus
        self.bus = self.player.get_bus()
        self.bus.add_signal_watch()

        self.bus.connect("message::eos", self.on_eos)
        self.bus.connect("message::tag", self.on_tag)
        self.bus.connect("message::error", self.on_error)
        self.bus.connect("message::state-changed",
                         self.on_message_state_changed)
        
        # dbus

        self.session_bus = dbus.SessionBus()
        self.bus_name = dbus.service.BusName("org.WRMS", self.session_bus)
        dbus.service.Object.__init__(self, self.bus_name, '/org/WRMS/Player')
        
        self.loop = GObject.MainLoop()
        self.loop.run()
        
    def on_eos(self, bus, msg):
        logging.warning("eos message received!")
        self.player.set_state(Gst.State.NULL)
        self.next()

    def on_tag(self, bus, msg):
        self.current_song_taglist = msg.parse_tag()
        logging.warning("tags recieved and saved")

    def on_error(self, bus, msg):
        logging.error(msg.parse_error())
        ret = self.player.set_state(Gst.State.NULL)
        logging.info("playbin state set to NULL!")
    
    def on_message_state_changed(self, bus, msg):
        logging.debug(str(msg.type))

    def urify(self, song):
        if isinstance(song, Song):
            return "file://" + music_dir + '/' + song.get_name()
        elif isinstance(song, str):
            return "file://" + music_dir + '/' + song
        return ""
    
    def pop_next_song(self):
        self.current_song = self.queue.pop()
        logging.debug("song popped")

        
    ############################### controls #################################

    @dbus.service.method("org.WRMS.player",
                         in_signature='', out_signature='')
    def play(self):
        if self.current_song == None:
            self.next()
        self.player.set_property("uri", self.urify(self.current_song))
        ret = self.player.set_state(Gst.State.PLAYING)
        logging.info(str(ret))
        logging.info("player playing")

    @dbus.service.method("org.WRMS.player",
                         in_signature='', out_signature='')
    def next(self):
        ret = self.player.set_state(Gst.State.NULL)
        logging.info(str(ret))
        self.pop_next_song()
        if self.current_song == None:
            ret = self.player.set_state(Gst.State.NULL)
            logging.info(str(ret))
            logging.warning("No songs left in the queue")
        else:
            self.play()

    @dbus.service.method("org.WRMS.player",
                         in_signature='', out_signature='')
    def pause(self):
        ret = self.player.set_state(Gst.State.PAUSED)
        logging.info(str(ret))
        logging.info("player paused")
        
    @dbus.service.method("org.WRMS.player",
                         in_signature='', out_signature='')
    def quit(self):
        self.player.set_state(Gst.State.NULL)
        logging.warning("shut down player; exit mainloop")
        self.loop.quit()

    ########################### extended controls ###########################

    @dbus.service.method("org.WRMS.player",
                         in_signature='s', out_signature='')
    def add_song(self, song):
        if not self.queue.append(song):
            logging.info("song added: " +  self.urify(song))
        else:
            logging.warning(song + " can't be loaded: no file associated")

    @dbus.service.method("org.WRMS.player",
                         in_signature='s', out_signature='')
    def upvote_song(self, song):
        if not self.queue.upvote_song(song):
            logging.info(str(song) + "upvoted")
        else:
            logging.warning("can't upvote " + str(song))

    @dbus.service.method("org.WRMS.player",
                         in_signature='s', out_signature='')    
    def downvote_song(self, song):
        if not self.queue.downvote_song(song):
            logging.info(str(song) + "downvoted")
        else:
            logging.warning("can't downvote " + str(song))

    def get_queue(self):
        return self.queue.get_list_of_all()
        
    @dbus.service.method("org.WRMS.player",
                         in_signature='', out_signature='a(s(sss))')
    def get_queue_raw(self):
        return self.queue.get_list_of_all_raw()

    @dbus.service.method("org.WRMS.player",
                         in_signature='', out_signature='s')
    def get_queue_string(self):
        return str(self.queue.get_list_of_all_raw())

    @dbus.service.method("org.WRMS.player",
                         in_signature='', out_signature='s')
    def get_current_metadata(self):
        return str(self.current_song_taglist.tostring())


if __name__ == "__main__":
    p = Player()
