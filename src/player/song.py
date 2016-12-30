from comparableMixin import ComparableMixin


class Song(ComparableMixin):
    path = ""
    weight, upvotes, downvotes = (0,0,0)

    def __init__(self, path):
        self.path = path

    def _cmpkey(self):
        return self.weight, self.path

    def __repr__(self):
        return "(Song: {0}; Weight: {1})".format(self.path, self.weight)

    def upvote(self):
        self.weight += 1
        self.upvotes += 1

    def downvote(self):
        self.weight -= 1
        self.downvotes += 1

    def get_weight(self):
        return self.weight
        
    def get_votes(self):
        return self.upvotes, self.downvotes
        
    def get_name(self):
        return self.path

    def get_all(self):
        return (self.path, (self.weight, self.upvotes, self.downvotes))
