import pytest

from django.urls import reverse
from rest_framework import status
from core import models
from ddf import G


@pytest.mark.django_db(transaction=True)
def test_get_workers(client, authorization_header):
    COUNT = 10
    [G(models.WorkerGroup) for i in range(COUNT)]
    resp = client.get(reverse('api:worker_group'), headers=authorization_header)
    assert resp.status_code == status.HTTP_200_OK
    assert resp.json()['count'] == models.WorkerGroup.objects.count() == COUNT


@pytest.mark.django_db(transaction=True)
def test_update_worker_group(client, authorization_header):
    worker_group = G(models.WorkerGroup)
    worker = G(models.Worker, group=None)
    resp = client.patch(
        reverse('api:worker', kwargs={'pk': worker.guid}),
        data={
            'group': worker_group.id,
        },
        content_type="application/json",
        headers=authorization_header
    )
    assert resp.status_code == status.HTTP_200_OK
    assert resp.json()['group'] == worker_group.id

    resp = client.patch(
        reverse('api:worker', kwargs={'pk': worker.guid}),
        data={
            'group': None,
        },
        content_type="application/json",
        headers=authorization_header
    )
    assert resp.status_code == status.HTTP_200_OK
    assert resp.json()['group'] is None


@pytest.mark.django_db(transaction=True)
def test_remove_worker(client, default_user, authorization_header):
    worker = G(models.Worker)
    resp = client.delete(
        reverse('api:worker', kwargs={'pk': worker.guid}),
        headers=authorization_header
    )
    assert resp.status_code == status.HTTP_204_NO_CONTENT
    assert models.WorkerGroup.objects.filter(id=worker.guid).count() == 0
