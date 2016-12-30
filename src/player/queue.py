import heapq
import logging

from song import Song

class Dyn_Queue:

    queue = []
    
    def __init__(self, *song):
        # parsing the parameters
        for s in song:
            self.queue.append(s)
        heapq._heapify_max(self.queue)
        logging.info("playlist created with " + str(self.get_list_of_all())
                     + " as start value")

    def delete(self, song):
        """Delete a song from the queue"""
        for s in self.queue:
            # checking if ether $song or a $song.name is in the queue
            if s == song or s.get_name() == song :
                self.queue.remove(s)
                # building a new heap after deleting
                heapq._heapify_max(self.queue)
                logging.debug(song + "deleted => " +  str(self.get_list_of_all()))
                return 0
        return 1

    def pop(self):
        """Remove and return the first song from the queue"""
        if len(self.queue) > 0:
            return heapq._heappop_max(self.queue)
        else:
            return None

    def append(self, new_song):
        """Append a song to the queue"""
        if isinstance(new_song, Song):
            self.queue.append(new_song)
        else:
            try:
                self.queue.append(Song(new_song))
            except IOError:
                logging.error("can't create song: no file associated to " + new_song)
                return 1
            heapq._heapify_max(self.queue)
            return 0

    def upvote_song(self, song):
        """Upvote song in the queue"""
        res = 1
        for s in self.queue:
            if s == song or s.get_name() == song:
                s.upvote()
                res = 0
                break
        # building a new hash with the changed ranks
        heapq._heapify_max(self.queue)
        return res
    
    def downvote_song(self, song):
        """Downvote song in the queue"""
        res = 1
        for s in self.queue:
            if s == song or s.get_name() == song:
                s.downvote()
                res = 0
                break
        heapq._heapify_max(self.queue)
        return res

    def get_list_of_all(self):
        """Return a list of all songs"""
        return heapq.nlargest(len(self.queue), self.queue)
        
    def get_list_of_all_raw(self):
        """Return a list of all songs reprsented by a tuple of their attributes"""
        return [x.get_all() for x in heapq.nlargest(len(self.queue), self.queue)]

    def __repr__(self):
        return str(self.queue)
