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

    # delete the first appereance of $song in the queue
    def delete(self, song):
        for s in self.queue:
            # checking if ether $song or a $song.name is in the queue
            if s == song or s.get_name() == song :
                self.queue.remove(s)
                # building a new heap after deleting
                heapq._heapify_max(self.queue)
                logging.debug(song + "deleted => " +  str(self.get_list_of_all()))
                return 0
        return 1

    # pops the first song in the queue
    def pop(self):
        if len(self.queue) > 0:
            return heapq._heappop_max(self.queue)
        else:
            logging.warning("Can't pop Song: Playlist is empty!")
            return None

    # append $song to the queue 
    def append(self, new_song):
        if isinstance(new_song, Song):
            self.queue.append(new_song)
        else:
            # catching a possible error while creating a new Song_object
            try:
                self.queue.append(Song(new_song))
            except IOError:
                logging.error("can't create song: no file associated to " + new_song)
                return 1
            heapq._heapify_max(self.queue)
            return 0

    # upvote $song
    def upvote_song(self, song):
        res = 1
        for s in self.queue:
            if s == song or s.get_name() == song:
                s.upvote()
                res = 0
                break
        # building a new hash with the changed ranks
        heapq._heapify_max(self.queue)
        return res
    
    # downvote $song
    def downvote_song(self, song):
        res = 1
        for s in self.queue:
            if s == song or s.get_name() == song:
                s.downvote()
                res = 0
                break
        heapq._heapify_max(self.queue)
        return res

    # return a list of all songs in the queue
    def get_list_of_all(self):
        return heapq.nlargest(len(self.queue), self.queue)
        
    # returns every song represented by a tuple of its attributes 
    def get_list_of_all_raw(self):
        return [x.get_all() for x in heapq.nlargest(len(self.queue), self.queue)]

    def __repr__(self):
        return str(self.queue)
