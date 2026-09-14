import ipaddress
import pytest
import random

from django.urls import reverse
from rest_framework import status
from core import models, utils
from ddf import G


def create_cidrs(client, authorization_header):
    worker_group = G(models.WorkerGroup)
    # TODO: must be uniq for at least test_get_cidrs
    cidr = f"{ipaddress.IPv4Address(random.randint(0, 2**32 - 1))}/32"
    resp = client.post(
        reverse('api:cidr'),
        data={
            'cidrs': cidr,
            'worker_group': worker_group.id,
        },
        headers=authorization_header
    )
    assert resp.status_code == status.HTTP_201_CREATED
    return cidr, worker_group


@pytest.mark.django_db(transaction=True)
def test_get_cidrs(client, authorization_header):
    utils.rds.flushdb()
    COUNT = 10
    for i in range(COUNT):
        create_cidrs(client, authorization_header)
    resp = client.get(reverse('api:cidr'), headers=authorization_header)
    assert resp.status_code == status.HTTP_200_OK
    assert resp.json()['count'] == COUNT


@pytest.mark.django_db(transaction=True)
def test_create_cidrs(client, authorization_header):
    utils.rds.flushdb()
    worker_group = G(models.WorkerGroup)
    resp = client.post(
        reverse('api:cidr'),
        data={
            'cidrs': '127.0.0.1/32',
            'worker_group': worker_group.id,
        },
        headers=authorization_header
    )
    assert resp.status_code == status.HTTP_201_CREATED


@pytest.mark.django_db(transaction=True)
def test_update_cidr(client, authorization_header):
    utils.rds.flushdb()
    cidr, _ = create_cidrs(client, authorization_header)
    worker_group = G(models.WorkerGroup)
    resp = client.patch(
        reverse('api:cidr', kwargs={'cidr': cidr}),
        data={'worker_group': worker_group.id},
        content_type="application/json",
        headers=authorization_header
    )
    assert resp.status_code == status.HTTP_200_OK


@pytest.mark.django_db(transaction=True)
def test_delete_cidr(client, authorization_header):
    utils.rds.flushdb()
    cidr, _ = create_cidrs(client, authorization_header)
    resp = client.delete(
        reverse('api:cidr', kwargs={'cidr': cidr}),
        headers=authorization_header
    )
    assert resp.status_code == status.HTTP_204_NO_CONTENT
