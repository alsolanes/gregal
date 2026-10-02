"""App principal."""
from util import slugify


def title_url(title):
    return "/posts/" + slugify(title)
