import logging

from sdk.errors import SpaceHTTPError, SpaceParsingError
from sdk.incident import Incident
from sdk.recipe import Recipe, RecipeStatus
from sdk.services import Space

logger = logging.getLogger(__name__)

def handler(incident: Incident, recipe: Recipe):
    """Create Jira Issue Recipe."""
    logger.info("Create a space, add user and post analysis")

    space = Space()
    results = recipe.results
    try:
        roomId = space.create_space(incident.data)
        results.log(f"Space created successfully\n")
        response = space.add_user(incident.data, roomId)
        results.log(response)
        space.post_analysis(incident.data, roomId)
        results.log(f"Analysis posted successfully\n")
        results.status = RecipeStatus.SUCCESSFUL
    except (SpaceHTTPError, SpaceParsingError) as e:
        results.log(f"Failed to create space: {e}")
        results.status = RecipeStatus.FAILED

def main():
    Recipe("space", handler).run()

if __name__ == "__main__":
    main()
