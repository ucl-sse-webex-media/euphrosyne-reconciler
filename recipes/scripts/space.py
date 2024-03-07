import logging

from sdk.errors import SpaceHTTPError, SpaceParsingError
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
        roomId = space.create_room(incident.data)
        log = f"Space created successfully: {roomId}"
        response = space.add_user(incident.data, roomId)
        log += response
        response = space.post_analysis(incident.data, roomId)
        response = f"Analysis posted successfully" {response}"
        log += response
        results.status = RecipeStatus.SUCCESSFUL
        results.log(log)
    except (SpaceHTTPError, SpaceParsingError) as e:
        results.log(f"Failed to create space: {e}")
        results.status = RecipeStatus.FAILED

    def main():
        Recipe("space", handler).run()

    if __name__ == "__main__":
        main()
