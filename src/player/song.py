import os, sys
import logging

sys.path.insert(0,"..")
import WRMSconf
from comparableMixin import ComparableMixin

music_dir = WRMSconf.music_dir

class Song(ComparableMixin):
    path = ""
    weight, upvotes, downvotes = (0,0,0)

    def __init__(self, path):
        if not os.path.isfile(music_dir + '/' + path):
            raise IOError("no file associated")
        self.path = path
        logging.debug(str(self) + "created")

    def _cmpkey(self):
        return self.weight, self.path

    def __repr__(self):
        return "(Song: {0}; Weight: {1})".format(self.path, self.weight)
	# Printing the rank (not the weight) should now be done by the playlist

    def upvote(self):
        self.weight += 1
        self.upvotes += 1
        logging.debug(str(self) + " upvoted")

    def downvote(self):
        self.weight -= 1
        self.downvotes += 1
        logging.debug(str(self) + " downvoted")

    def get_weight(self):
        return self.weight
        
    def get_votes(self):
        return self.upvotes, self.downvotes
        
    def get_name(self):
        return self.path

    def get_all(self):
        return (self.path, (self.weight, self.upvotes, self.downvotes))
