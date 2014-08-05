import heapq
import logging

from song import Song

class Dyn_Queue:

    queue = []
    
    def __init__(self, *song):
        for s in song:
            self.queue.append(s)
        logging.info("playlist created with " + str(self.get_list_of_all())
                     + " as start value")

    def delete(self, song):
        for s in self.queue:
            if s == song or s.get_name() == song :
                self.queue.remove(s)
                heapq.heapify(self.queue)
                logging.debug(song + "deleted => " +  str(self.get_list_of_all()))
                return 0
        return 1

    def pop(self):
        if len(self.queue) > 0:
            return heapq.heappop(self.queue)
        else:
            logging.warning("Can't pop Song: Playlist is empty!")
            return None

    def append(self, new_song):
        if isinstance(new_song, Song):
            heapq.heappush(self.queue, new_song)
            return 0
        else:
            try:
                heapq.heappush(self.queue, Song(new_song))
                return 0
            except IOError:
                logging.error("can't create song: no file associated to " + new_song)
                return 1

    def upvote_song(self, song):
        for s in self.queue:
            if s == song or s.get_name() == song:
                s.upvote()
                return 0
        heapq.heapify(self.queue)
        return 1
    
    def downvote_song(self, song):
        res = 0
        for s in self.queue:
            if s == song or s.get_name() == song:
                s.downvote()
                return 0
        heapq.heapify(self.queue)
        return 1

    def get_list_of_all(self):
        return heapq.nlargest(len(self.queue), self.queue)
        
    # returns every song represented by a tuple of its attributes 
    def get_list_of_all_raw(self):
        return [x.get_all() for x in heapq.nlargest(len(self.queue), self.queue)]

    def __repr__(self):
        return str(self.queue)
