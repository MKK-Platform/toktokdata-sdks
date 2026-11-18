"""API namespaces. Each class is reached through a :class:`toktokdata.TokTokData` attribute."""

from .account import Account
from .agencies import Agencies
from .alerts import Alerts
from .creators import Creators
from .exports import Exports
from .gifters import Gifters
from .gifts import Gifts
from .insights import Insights
from .live import Live
from .rankings import Rankings
from .trends import Trends
from .videos import Videos

__all__ = [
    "Account",
    "Agencies",
    "Alerts",
    "Creators",
    "Exports",
    "Gifters",
    "Gifts",
    "Insights",
    "Live",
    "Rankings",
    "Trends",
    "Videos",
]
