Feature: Test the behaviors of *sql.Tx

    Scenario Outline: Create a customer then rollback
        Given I <mode> database preparer
        And there are no rows in table "customer" of database "default"
        Then no rows are available in table "customer" of database "default"

        Given I start a new transaction

        When I create a new customer:
            """
            {
                "id": 2,
                "country": "US",
                "first_name": "Jane",
                "last_name": "Doe",
                "email": "jane.doe@example.com",
                "created_at": "2022-02-22T20:00:02Z"
            }
            """
        Then I got no error

        When I find a customer by id "2"

        Then I got no error
        And I got a customer:
            """
            {
                "id": 2,
                "country": "US",
                "first_name": "Jane",
                "last_name": "Doe",
                "email": "jane.doe@example.com",
                "created_at": "2022-02-22T20:00:02Z",
                "updated_at": "<ignore-diff>"
            }
            """

        When I rollback the transaction
        Then no rows are available in table "customer" of database "default"

        Examples:
            | mode       | ignore-diff   |
            | use        | <ignore-diff> |
            | do not use | <ignore-diff> |

    Scenario Outline: CRUD and then commit
        Given now is "2022-12-22T20:00:02Z"
        And I <mode> database preparer
        And there are no rows in table "customer" of database "default"
        Then no rows are available in table "customer" of database "default"

        Given I start a new transaction

        When I create a new customer:
            """
            {
                "id": 1,
                "country": "US",
                "first_name": "John",
                "last_name": "Doe",
                "email": "john.doe@example.com",
                "created_at": "2022-02-22T20:00:02Z"
            }
            """
        Then I got no error

        When I find a customer by id "1"
        Then I got no error
        And I got a customer:
            """
            {
                "id": 1,
                "country": "US",
                "first_name": "John",
                "last_name": "Doe",
                "email": "john.doe@example.com",
                "created_at": "2022-02-22T20:00:02Z",
                "updated_at": "<ignore-diff>"
            }
            """

        When I create a new customer:
            """
            {
                "id": 2,
                "country": "US",
                "first_name": "Jane",
                "last_name": "Doe",
                "email": "jane.doe@example.com",
                "created_at": "2022-02-22T20:00:02Z"
            }
            """
        Then I got no error

        When I find all customers
        Then I got no error
        And I got these customers:
            """
            [
                {
                    "id": 1,
                    "country": "US",
                    "first_name": "John",
                    "last_name": "Doe",
                    "email": "john.doe@example.com",
                    "created_at": "2022-02-22T20:00:02Z",
                    "updated_at": "<ignore-diff>"
                },
                {
                    "id": 2,
                    "country": "US",
                    "first_name": "Jane",
                    "last_name": "Doe",
                    "email": "jane.doe@example.com",
                    "created_at": "2022-02-22T20:00:02Z",
                    "updated_at": "<ignore-diff>"
                }
            ]
            """

        When I delete a customer by id "1"
        Then I got no error

        When I update a customer by id "2":
            """
            {
                "country": "US",
                "first_name": "Modified",
                "last_name": "Modified",
                "email": "jane.doe@example.com",
                "created_at": "2022-02-22T20:00:02Z"
            }
            """
        Then I got no error

        When I commit the transaction
        And only these rows are available in table "customer" of database "default"
            | id | country | first_name | last_name | email                | created_at           | updated_at           |
            | 2  | US      | Modified   | Modified  | jane.doe@example.com | 2022-02-22T20:00:02Z | 2022-12-22T20:00:02Z |

        Examples:
            | mode       | ignore-diff   |
            | use        | <ignore-diff> |
            | do not use | <ignore-diff> |
