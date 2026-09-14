import redis
from django.conf import settings


rds = redis.Redis.from_url(
    settings.REDIS_URL,
    decode_responses=True,
)
