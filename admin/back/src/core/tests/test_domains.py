import string
import pytest
import random

from django.urls import reverse
from rest_framework import status
from core import models, utils
from ddf import G


def random_domain():
    length = random.randint(5, 15)
    name = ''.join(random.choices(string.ascii_lowercase, k=length))
    return f"{name}.example.com"


def create_domains(client, authorization_header):
    worker_group = G(models.WorkerGroup)
    # TODO: must be uniq for at least test_get_domains
    pattern = random_domain()
    resp = client.post(
        reverse('api:domain'),
        data={
            'patterns': pattern,
            'worker_group': worker_group.id,
        },
        headers=authorization_header
    )
    assert resp.status_code == status.HTTP_201_CREATED
    return pattern, worker_group


@pytest.mark.django_db(transaction=True)
def test_get_domains(client, authorization_header):
    utils.rds.flushdb()
    COUNT = 10
    for i in range(COUNT):
        create_domains(client, authorization_header)
    resp = client.get(reverse('api:domain'), headers=authorization_header)
    assert resp.status_code == status.HTTP_200_OK
    assert resp.json()['count'] == COUNT


@pytest.mark.django_db(transaction=True)
def test_create_domain(client, authorization_header):
    utils.rds.flushdb()
    worker_group = G(models.WorkerGroup)
    resp = client.post(
        reverse('api:domain'),
        data={
            'patterns': 'test.com',
            'worker_group': worker_group.id,
        },
        headers=authorization_header
    )
    assert resp.status_code == status.HTTP_201_CREATED


@pytest.mark.django_db(transaction=True)
def test_update_domain(client, authorization_header):
    utils.rds.flushdb()
    pattern, _ = create_domains(client, authorization_header)
    worker_group = G(models.WorkerGroup)
    resp = client.patch(
        reverse('api:domain', kwargs={'pattern': pattern}),
        data={'worker_group': worker_group.id},
        content_type="application/json",
        headers=authorization_header
    )
    assert resp.status_code == status.HTTP_200_OK


@pytest.mark.django_db(transaction=True)
def test_delete_domain(client, authorization_header):
    utils.rds.flushdb()
    pattern, _ = create_domains(client, authorization_header)
    resp = client.delete(
        reverse('api:domain', kwargs={'pattern': pattern}),
        headers=authorization_header
    )
    assert resp.status_code == status.HTTP_204_NO_CONTENT
