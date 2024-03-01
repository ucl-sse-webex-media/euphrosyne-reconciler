import logging

from sdk.incident import Incident
from sdk.recipe import Recipe, RecipeStatus
from sdk.services import Space

logger = logging.getLogger(__name__)

#TODO: request format from bot
"""
{
    "title": "New Space",
    "description": "This is a new space",
    "personId": "12345",
}
"""

def handler(incident: Incident, recipe: Recipe):
    """Create Jira Issue Recipe."""
    # QUESTION: 1. creating a room need title (compulsory) + (description), where does this come from? use default
    # or provide a form
    logger.info("Create a space, add user and post analysis")

    space = Space()
    results = recipe.results
    try:
        # create a new space
        room = space.create_room(incident.data)
        #TODO: add error handling
        roomId = room["id"]
        results.log(f"Space created successfully: {roomId}")
        #TODO: add the user who send the reques to this new space, post previous
        space.add_user(incident.data, roomId)

    # analysis there


def main():
    Recipe("space", handler).run()

if __name__ == "__main__":
    main()
