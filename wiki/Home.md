# Remiges Alya

Remiges Alya is an open source web services development framework for Go programmers. It allows programmers to rapidly write web service calls as per our standards and conventions.

## Why is Alya needed

The proven and powerful [Gin](https://gin-gonic.com/) framework for writing web service calls in Go is already very popular. We have built Alya on top of Gin, without modifying Gin. Alya is necessary because it provides lots of helper functions and other hooks to allow the programmer to build secure, bug-free web service calls without having to re-invent various wheels. All these things could have been developed by you too, but it makes no sense developing this set of components for every project. What we have added:
* middleware which plugs into Gin and intercepts each incoming request and validates the token against [Remiges IDshield](https://github.com/remiges-tech/idshield/wiki) or [Keycloak](https://www.keycloak.org/). Token validation is therefore automatic, and uses Redis to cache valid tokens if Redis is available, reducing load on IDshield. If the token is invalid, the call does not even reach the business logic.
* extensions to [a popular validator library](https://github.com/go-playground/validator) to help validate new data types which the base Validator does not support. For instance, mobile number validation is now possible by integrating with [Google's `libphonenumber`](https://github.com/google/libphonenumber). This allows the programmer to validate all the common types of parameters which the request receives. We're adding new data types every day.
* a library to build the response data structure which the request will return, in the data structure and format we use for our projects. This response format includes support for multi-lingual messages.
* support for long-running queries. Alya comes with [RabbitMQ](https://www.rabbitmq.com/) integration and a framework of asynchronous worker threads to distribute long-running queries and requests across all the servers in your cluster. The programmer can write a web service call which receives the request, queues it in the RabbitMQ work queue for one of the worker threads to pick up, while it returns a come-back-later response to the caller. Later, when the processing is complete, the caller can poll the server and pick up the results. The programmer will not need to write any code to interface with RabbitMQ *etc* This allows the administrator to decide how many asynchronous worker threads she wants to run on each server, and split the load of synchronous and asynchronous processing optimally.
* support for batch processing. Alya makes it very easy for the programmer to write code to receive a batch (in a table or file), break up the batch into smaller chunks, and push out each chunk to be processed by asynchronous threads. The RabbitMQ message bus is used here too, and the programmer does not need to bother with the framework. She just writes the processing code as a function which the asynchronous worker thread will call. All the standard operating procedures and tools to track the completion of each job in a batch, aggregate the results, and finally allow the caller to pick it up, are part of Alya.

## The Alya eco-system

Alya requires Gin and the validator library -- these are mandatory. The following are optional third-party components or frameworks which enhance Alya's effectiveness and power, and are directly or indirectly called from your code
* [Remiges IDshield](https://github.com/remiges-tech/idshield/wiki) or [Keycloak](https://www.keycloak.org/) to handle token verification and session tracking, and authorisation
* [Remiges LogHarbour](https://github.com/remiges-tech/logharbour/wiki) for three types of logging. Internally LogHarbour uses Kafka and ElasticSearch
* [Remiges Rigel](https://github.com/remiges-tech/rigel/wiki) for configuration information
* [Redis](https://redis.io/) for a variety of performance enhancing caching
* [RabbitMQ](https://rabbitmq.com) in case you wish to use the support for long-running queries and batch processing
* [sqlc](https://sqlc.dev/) for accessing relational databases. This is only necessary if your application needs an RDB.

Alya today is available only for Go programmers. The roadmap includes releasing a second version on Java SpringBoot. This version will use Java native frameworks in place of Gin and the Validator library.

## The philosophy

Programmers writing web service calls in any programming language should be able to spend most of their time writing business logic. They should not have to spend time
* implementing a `user` table and writing user-add, user-delete operations, password-change operations
* designing an authorisation framework and a `roles` table
* ensuring that their regex validates phone numbers or credit card numbers, *etc*
* thinking up a way to log the application's activities
* parsing the input request JSON for mandatory and optional fields, validating each parameter
* struggling with asynchronous threads and batch tracking to implement long-running reports and queries

All of these are completely orthogonal to the actual business logic of any mid or large business application, yet are needed everytime. The aim behind Alya was to standardise one good answer to each of these questions, and free the programmer's mind to let her focus on the actual business logic.

This means that we have One True Alya Way to do most of these things. We accept call requests in JSON, not in XML. Responses from calls always carry their data payload in a specific format, and carry errors in another format. User session tracking is done using OAuth2, *via* Keycloak, and not through any cookies and a `sessions` table. These are embedded in the design of Alya and are not flexible. We have about a decade of writing web service calls to say that the Alya Way to do things makes for robust and efficient applications.

## Open source

Remiges Alya is the intellectual property of Remiges Technologies Pvt Ltd. It is being made available to you under the terms of the [Apache Licence 2.0](https://opensource.org/license/apache-2-0/).

We build products which we use as part of the solutions we build for our clients. We are committed to maintaining them because these are our critical assets which form our technical edge. We will be happy to offer consultancy and professional services around our products, and will be thrilled to see the larger community use our products with or without our direct participation. We welcome your queries, bug reports and pull requests.

## Remiges Technologies 

[Remiges Technologies Pvt Ltd](https://remiges.tech) ("Remiges") is a private limited company incorporated in India and controlled under the Companies Act 2013 (MCA), India. Remiges is a technology-driven software projects company whose vision is to build the world's best business applications for enterprise clients, using talent, thought and technology. Remiges views themselves as a technology leader who execute projects with a product engineering mindset, and has a strong commitment to the open source community. Our clients include India's three trading exchanges (who are among the largest trading exchanges in the world in terms of transaction volumes), some of the top ten broking houses in India, both of India's securities depositories, Fortune 500 MNC manufacturing organisations, cloud-first technology startups, and others.