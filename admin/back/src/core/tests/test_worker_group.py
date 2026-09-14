import pytest

from django.urls import reverse
from rest_framework import status
from django.conf import settings
from core import models
from ddf import G


@pytest.mark.django_db(transaction=True)
def test_get_worker_groups(client, authorization_header):
    G(models.WorkerGroup)
    resp = client.get(reverse('api:worker_group'), headers=authorization_header)
    assert resp.status_code == status.HTTP_200_OK
    assert resp.json()['count'] == models.WorkerGroup.objects.count()


@pytest.mark.django_db(transaction=True)
def test_create_worker_group(client, authorization_header):
    GROUP_NAME = 'test_group'
    resp = client.post(
        reverse('api:worker_group'),
        data={'group_name': GROUP_NAME},
        headers=authorization_header
    )
    assert resp.status_code == status.HTTP_201_CREATED
    assert models.WorkerGroup.objects.filter(group_name=GROUP_NAME).count() == 1


@pytest.mark.django_db(transaction=True)
def test_wrong_create_worker_group(client, authorization_header):
    resp = client.post(
        reverse('api:worker_group'),
        data={'group_name': settings.DEFAULT_GROUP_NAME},
        headers=authorization_header
    )
    assert resp.status_code == status.HTTP_400_BAD_REQUEST


@pytest.mark.django_db(transaction=True)
def test_update_worker_group(client, authorization_header):
    GROUP_NAME = 'test_group'
    worker_group = G(models.WorkerGroup)
    resp = client.patch(
        reverse('api:worker_group', kwargs={'pk': worker_group.id}),
        data={
            'group_name': GROUP_NAME,
        },
        content_type="application/json",
        headers=authorization_header
    )
    assert resp.status_code == status.HTTP_200_OK
    assert GROUP_NAME != worker_group.group_name  # cos didn't refresh


@pytest.mark.django_db(transaction=True)
def test_wrong_update_worker_group(client, authorization_header):
    worker_group = G(models.WorkerGroup)
    resp = client.patch(
        reverse('api:worker_group', kwargs={'pk': worker_group.id}),
        data={
            'group_name': settings.DEFAULT_GROUP_NAME,
        },
        content_type="application/json",
        headers=authorization_header
    )
    assert resp.status_code == status.HTTP_400_BAD_REQUEST


@pytest.mark.django_db(transaction=True)
def test_remove_worker_group(client, default_user, authorization_header):
    worker_group = G(models.WorkerGroup)
    resp = client.delete(
        reverse('api:worker_group', kwargs={'pk': worker_group.id}),
        headers=authorization_header
    )
    assert resp.status_code == status.HTTP_204_NO_CONTENT
    assert models.WorkerGroup.objects.filter(id=worker_group.id).count() == 0
