import pytest
import string
import random

from django.conf import settings
from django.contrib.auth import get_user_model
from django.urls import reverse
from rest_framework import status


def pytest_configure():
    settings.PASSWORD_HASHERS = [
        'django.contrib.auth.hashers.MD5PasswordHasher',
    ]


def generate_random_password(length):
    if length <= 0:
        return "Password length must be a positive integer."

    # Define the set of characters to choose from
    all_characters = string.ascii_letters + string.digits + string.punctuation

    # Generate the password by randomly choosing characters
    password = ''.join(random.choices(all_characters, k=length))
    return password


User = get_user_model()
USER_PASSWORD = generate_random_password(8)
USER_USERNAME = 'main_profile'


@pytest.fixture(autouse=True)
def assert_empty_output(capfd):
    yield

    captured = capfd.readouterr()

    assert captured.out == ''
    assert captured.err == ''


@pytest.fixture(autouse=True)
def default_user():
    user = User.objects.create(
        username=USER_USERNAME,
        is_superuser=True
    )
    user.set_password(USER_PASSWORD)
    user.save()
    return user


@pytest.fixture
def authorization_token(client):
    data = {
        'username': USER_USERNAME,
        'password': USER_PASSWORD
    }
    resp = client.post(reverse('api:get_token'), data=data, content_type="application/json")
    assert resp.status_code == status.HTTP_200_OK
    return resp.json()['token']


@pytest.fixture
def authorization_header(client, authorization_token):
    return {'Authorization': f'Token {authorization_token}'}
