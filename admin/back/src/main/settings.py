from pathlib import Path
import environ

# Build paths inside the project like this: BASE_DIR / 'subdir'.
BASE_DIR = Path(__file__).resolve().parent.parent

# SECURITY WARNING: don't run with debug turned on in production!
env = environ.Env(
    DEBUG=(bool, False),
    DJANGO_SHOW_SWAGGER=(bool, False),
    DJANGO_STATIC_ROOT=(str, ''),
    DJANGO_CSRF_TRUSTED_ORIGINS=(str, 'https://*'),
)

SHOW_SWAGGER = env('DJANGO_SHOW_SWAGGER')
SECRET_KEY = env('DJANGO_SECRET_KEY')
DEBUG = env('DEBUG')
ALLOWED_HOSTS = [env('DJANGO_ALLOWED_HOSTS')]
CSRF_TRUSTED_ORIGINS = [env('DJANGO_CSRF_TRUSTED_ORIGINS')]

PROJECT_APPS = [
    'core',
]

INSTALLED_APPS = [
    'django.contrib.admin',
    'django.contrib.auth',
    'django.contrib.contenttypes',
    'django.contrib.sessions',
    'django.contrib.messages',
    'django.contrib.staticfiles',
    'rest_framework',
    'rest_framework.authtoken',
    'drf_spectacular',
] + PROJECT_APPS

MIDDLEWARE = [
    'django.middleware.security.SecurityMiddleware',
    'django.contrib.sessions.middleware.SessionMiddleware',
    'django.middleware.common.CommonMiddleware',
    'django.middleware.csrf.CsrfViewMiddleware',
    'django.contrib.auth.middleware.AuthenticationMiddleware',
    'django.contrib.messages.middleware.MessageMiddleware',
    'django.middleware.clickjacking.XFrameOptionsMiddleware',
]

ROOT_URLCONF = 'main.urls'

TEMPLATES = [
    {
        'BACKEND': 'django.template.backends.django.DjangoTemplates',
        'DIRS': [],
        'APP_DIRS': True,
        'OPTIONS': {
            'context_processors': [
                'django.template.context_processors.request',
                'django.contrib.auth.context_processors.auth',
                'django.contrib.messages.context_processors.messages',
            ],
        },
    },
]

WSGI_APPLICATION = 'main.wsgi.application'

DATABASES = {
    'default': env.db_url('DATABASE_URL')
}

AUTH_PASSWORD_VALIDATORS = [
    {
        'NAME': 'django.contrib.auth.password_validation.UserAttributeSimilarityValidator',
    },
    {
        'NAME': 'django.contrib.auth.password_validation.MinimumLengthValidator',
    },
    {
        'NAME': 'django.contrib.auth.password_validation.CommonPasswordValidator',
    },
    {
        'NAME': 'django.contrib.auth.password_validation.NumericPasswordValidator',
    },
]

LANGUAGE_CODE = 'en-us'
TIME_ZONE = 'UTC'
USE_I18N = True
USE_TZ = True
STATIC_URL = '/backend-static/'
STATIC_ROOT = env('DJANGO_STATIC_ROOT')

AUTH_USER_MODEL = 'core.Profile'


MAILERS = {
    'default': {
        'BACKEND': 'django.core.mail.backends.console.EmailBackend',
    },
}

REST_FRAMEWORK = {
    'DEFAULT_SCHEMA_CLASS': 'main.schema.EmptyTagAuthoSchema',
    'DEFAULT_AUTHENTICATION_CLASSES': [
        "rest_framework.authentication.TokenAuthentication",
    ],
    'DEFAULT_PERMISSION_CLASSES': (
        'rest_framework.permissions.IsAuthenticated',
    ),
    'PREPROCESSING_HOOKS': ['main.schema.preprocessing_filter_spec'],
    'DEFAULT_PAGINATION_CLASS': 'core.pagination.CustomPageNumberPagination',
}

REDIS_URL = env('REDIS_URL')
DEFAULT_GROUP_NAME = env('DEFAULT_GROUP_NAME')
DOMAINS_REDIS_KEY = env('DOMAINS_REDIS_KEY')
CIDRS_REDIS_KEY = env('CIDRS_REDIS_KEY')
DEFAULT_ROUTE_REDIS_KEY = env('DEFAULT_ROUTE_REDIS_KEY')
