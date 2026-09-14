from core import models, utils
from django.conf import settings


def get_domain_routes(search: str | None = None):
    routes = utils.rds.hgetall(settings.DOMAINS_REDIS_KEY)

    groups = {
        group.group_name: group.pk
        for group in models.WorkerGroup.objects.all()
    }

    search = search.strip().lower() if search else None

    return [
        {
            "pattern": pattern,
            "group": groups.get(group_name),
        }
        for pattern, group_name in routes.items()
        if not search or search in pattern.lower()
    ]


def get_domain_route(pattern):
    return utils.rds.hget(settings.DOMAINS_REDIS_KEY, pattern)


def domain_route_exists(pattern):
    return utils.rds.hexists(settings.DOMAINS_REDIS_KEY, pattern)


def create_domain_route(patterns, worker_group):
    for pattern in patterns:
        utils.rds.hset(
            settings.DOMAINS_REDIS_KEY,
            pattern,
            worker_group.group_name,
        )
    return {
        'pattern': patterns,
        'worker_group': worker_group.pk

    }


def update_domain_route(pattern, worker_group):
    utils.rds.hset(
        settings.DOMAINS_REDIS_KEY,
        pattern,
        worker_group.group_name,
    )

    return {
        "pattern": pattern,
        "worker_group": worker_group.pk,
    }


def delete_domain_route(pattern):
    return utils.rds.hdel(
        settings.DOMAINS_REDIS_KEY,
        pattern,
    )


def get_cidr_routes(search: str | None = None):
    routes = utils.rds.hgetall(settings.CIDRS_REDIS_KEY)

    groups = {
        group.group_name: group.pk
        for group in models.WorkerGroup.objects.all()
    }

    search = search.strip().lower() if search else None

    result = []

    for cidr, group_name in routes.items():
        cidr = (
            cidr.decode("utf-8")
            if isinstance(cidr, bytes)
            else cidr
        )
        group_name = (
            group_name.decode("utf-8")
            if isinstance(group_name, bytes)
            else group_name
        )

        if search and search not in cidr.lower():
            continue

        result.append({
            "cidr": cidr,
            "worker_group": groups.get(group_name),
        })

    return result


def get_cidr_route(cidr):
    return utils.rds.hget(settings.CIDRS_REDIS_KEY, cidr)


def cidr_route_exists(cidr):
    return utils.rds.hexists(settings.CIDRS_REDIS_KEY, cidr)


def create_cidr_route(cidrs, worker_group):
    for cidr in cidrs:
        utils.rds.hset(
            settings.CIDRS_REDIS_KEY,
            cidr,
            worker_group.group_name,
        )
    return {
        'cidrs': cidrs,
        'worker_group': worker_group.pk
    }


def update_cidr_route(cidr, worker_group):
    utils.rds.hset(
        settings.CIDRS_REDIS_KEY,
        cidr,
        worker_group.group_name,
    )
    return {
        'cidr': cidr,
        'worker_group': worker_group.pk
    }


def delete_cidr_route(cidr):
    return utils.rds.hdel(
        settings.CIDRS_REDIS_KEY,
        cidr,
    )


def get_default_route():
    group_name = utils.rds.get(settings.DEFAULT_ROUTE_REDIS_KEY)

    if isinstance(group_name, bytes):
        group_name = group_name.decode("utf-8")

    if not group_name:
        return None

    if group_name == settings.DEFAULT_GROUP_NAME:
        return {"worker_group": 0}

    group = models.WorkerGroup.objects.filter(
        group_name=group_name
    ).first()

    if group is None:
        return None

    return {"worker_group": group.pk}


def update_default_route(worker_group):
    if worker_group == 0:
        utils.rds.set(settings.DEFAULT_ROUTE_REDIS_KEY, settings.DEFAULT_GROUP_NAME)
        return {"worker_group": 0}

    group = models.WorkerGroup.objects.get(pk=worker_group)

    utils.rds.set(settings.DEFAULT_ROUTE_REDIS_KEY, group.group_name)
    return {"worker_group": group.pk}
