{ pkgs, ... }:

{
  home.packages = with pkgs; [
    (python3.withPackages (ps: with ps; [
      # Package management
      pip
      setuptools
      wheel
      build
      
      # Web frameworks
      fastapi
      uvicorn
      django
      flask
      starlette
      
      # HTTP & WebSockets
      jinja2
      requests
      httpx
      aiohttp
      websockets
      
      # Database
      sqlalchemy
      asyncpg
      psycopg2
      psycopg
      alembic
      redis
      pymongo
      motor
      
      # Configuration
      python-dotenv
      pydantic
      pydantic-settings
      pyyaml
      toml
      
      # Data science
      numpy
      pandas
      scipy
      matplotlib
      seaborn
      scikit-learn
      
      # Jupyter
      notebook
      jupyterlab
      ipykernel
      ipython
      
      # GUI
      pyside6
      pyqt5
      pyqt6
      pillow
      
      # CLI utilities
      rich
      click
      typer
      loguru
      tqdm
      
      # Web utilities
      python-multipart
      
      # Code quality
      ruff
      black
      isort
      mypy
      pylint
      autopep8
      flake8
      
      # Testing
      pytest
      pytest-asyncio
      pytest-cov
      pytest-mock
      faker
      hypothesis
      
      # Message queues
      pika
      celery
      kombu
      
      # Async
      trio
      anyio
      
      # Performance
      httptools
      uvloop
      
      # Serialization
      msgpack
      orjson
      
      # Date/time
      python-dateutil
      pytz
      arrow
      
      # Security
      cryptography
      pyjwt
      passlib
      bcrypt
      
      # Observability
      prometheus-client
      opentelemetry-api
      
      # gRPC
      protobuf
      grpcio
      grpcio-tools
    ]))
  ];
}
